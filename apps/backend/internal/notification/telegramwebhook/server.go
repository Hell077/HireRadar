package telegramwebhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	notification "github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
)

type Handler interface {
	HandleMessage(context.Context, int64, int64, string, string, string) error
	HandleCallback(context.Context, int64, string, string) error
}

type Pinger interface{ Ping(context.Context) error }

type Update struct {
	Message *struct {
		Text string `json:"text"`
		From struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"message"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"callback_query"`
}

func New(secret string, bot Handler, db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if db == nil || db.Ping(r.Context()) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /telegram/webhook", func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || bot == nil || len(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")) != len(secret) || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		var update Update
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&update); err != nil {
			http.Error(w, "invalid update", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			http.Error(w, "invalid update body", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		if update.Message != nil {
			if err := bot.HandleMessage(ctx, update.Message.From.ID, update.Message.Chat.ID, update.Message.Chat.Type, update.Message.From.Username, update.Message.Text); err != nil {
				http.Error(w, "update failed", http.StatusInternalServerError)
				return
			}
		}
		if update.CallbackQuery != nil {
			if err := bot.HandleCallback(ctx, update.CallbackQuery.From.ID, update.CallbackQuery.ID, update.CallbackQuery.Data); err != nil && !errors.Is(err, notification.ErrInvalidCallback) && !errors.Is(err, notification.ErrFeedbackNotOwned) {
				http.Error(w, "update failed", http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
