package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"boiler-go/internal/repository/db"
	"boiler-go/internal/repository/repo"
	"boiler-go/pkg/logger"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	svix "github.com/svix/svix-webhooks/go"
)

type WebhookHandler struct {
	userRepo      *repo.UserRepo
	webhookSecret string
	log           zerolog.Logger
}

func NewWebhookHandler(pool *sqlx.DB, webhookSecret string, log zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{userRepo: repo.NewUserRepo(pool, log), webhookSecret: webhookSecret, log: log}
}

type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkUserData struct {
	ID             string `json:"id"`
	PrimaryEmailID string `json:"primary_email_address_id"`
	EmailAddresses []struct {
		ID           string `json:"id"`
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type clerkDeletedData struct {
	ID      string `json:"id"`
	Deleted *bool  `json:"deleted"`
}

// HandleClerk godoc
// @Summary		Clerk webhook
// @Description	Receives Clerk user lifecycle events (Svix-signed) and syncs local users.
// @Tags			webhooks
// @Accept			json
// @Produce		json
// @Param			svix-id			header		string	true	"Svix message ID"
// @Param			svix-timestamp	header		string	true	"Svix timestamp"
// @Param			svix-signature	header		string	true	"Svix signature"
// @Param			body			body		object	true	"Clerk event payload"
// @Success		200				{object}	handler.SuccessResponse
// @Failure		400				{object}	handler.ErrorResponse
// @Router			/webhooks/clerk [post]
func (h *WebhookHandler) HandleClerk(c echo.Context) error {
	log := logger.FromContext(c.Request().Context())

	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return NewEchoError(http.StatusBadRequest, "bad_request", "unable to read webhook body")
	}

	wh, err := svix.NewWebhook(h.webhookSecret)
	if err != nil {
		log.Error().Err(err).Msg("invalid webhook secret configuration")
		return NewEchoError(http.StatusInternalServerError, "internal_error", "webhook not configured")
	}
	if err := wh.Verify(payload, c.Request().Header); err != nil {
		log.Warn().Err(err).Msg("clerk webhook signature verification failed")
		return NewEchoError(http.StatusBadRequest, "bad_request", "invalid webhook signature")
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return NewEchoError(http.StatusBadRequest, "bad_request", "invalid webhook payload")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	switch event.Type {
	case "user.created", "user.updated":
		var data clerkUserData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return NewEchoError(http.StatusBadRequest, "bad_request", "invalid user payload")
		}
		if data.ID == "" {
			return NewEchoError(http.StatusBadRequest, "bad_request", "missing user id")
		}
		email := primaryEmail(data)
		if email == "" {
			log.Warn().Str("clerk_id", data.ID).Msg("clerk user has no email; skipping sync")
			return c.JSON(http.StatusOK, SuccessResponse{Success: true, Message: "ignored: no email"})
		}
		name := displayName(data, email)
		user, err := h.userRepo.UpsertByClerkID(ctx, db.UpsertUserByClerkIDParams{
			ClerkID: toNullText(data.ID),
			Email:   strings.ToLower(strings.TrimSpace(email)),
			Name:    strings.TrimSpace(name),
		})
		if err != nil {
			log.Error().Err(err).Str("clerk_id", data.ID).Msg("clerk user sync failed")
			return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to sync user")
		}
		return c.JSON(http.StatusOK, SuccessResponse{Success: true, Data: userResponse(user), Message: "user synced"})
	case "user.deleted":
		var data clerkDeletedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return NewEchoError(http.StatusBadRequest, "bad_request", "invalid user payload")
		}
		if data.ID == "" {
			return NewEchoError(http.StatusBadRequest, "bad_request", "missing user id")
		}
		if err := h.userRepo.DeleteByClerkID(ctx, data.ID); err != nil {
			// Deleting an already-absent user is not a webhook failure; ack it.
			log.Warn().Err(err).Str("clerk_id", data.ID).Msg("clerk user delete affected no rows")
			return c.JSON(http.StatusOK, SuccessResponse{Success: true, Message: "user already absent"})
		}
		return c.JSON(http.StatusOK, SuccessResponse{Success: true, Message: "user deleted"})
	default:
		log.Info().Str("event_type", event.Type).Msg("ignoring unhandled clerk event")
		return c.JSON(http.StatusOK, SuccessResponse{Success: true, Message: "ignored"})
	}
}

func toNullText(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func primaryEmail(data clerkUserData) string {
	if len(data.EmailAddresses) == 0 {
		return ""
	}
	if data.PrimaryEmailID != "" {
		for _, e := range data.EmailAddresses {
			if e.ID == data.PrimaryEmailID {
				return e.EmailAddress
			}
		}
	}
	return data.EmailAddresses[0].EmailAddress
}

func displayName(data clerkUserData, fallbackEmail string) string {
	name := strings.TrimSpace(strings.TrimSpace(data.FirstName) + " " + strings.TrimSpace(data.LastName))
	if name != "" {
		return name
	}
	if i := strings.Index(fallbackEmail, "@"); i > 0 {
		return fallbackEmail[:i]
	}
	return fallbackEmail
}
