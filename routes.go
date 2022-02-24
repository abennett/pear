package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
)

const (
	ApplicationJson = "application/json"
)

func NewRouter(pear *PearService) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(pear.VerifyRequest)

	r.Post("/new", HandleNew(pear))
	r.Post("/submit", HandleSubmit(pear))

	return r
}

func WriteMsg(w http.ResponseWriter, msg slack.Msg) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", ApplicationJson)
	_, err = w.Write(b)
	return err
}

func HandleNew(pear *PearService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := slack.SlashCommandParse(r)
		if err != nil {
			errorWrapper(w, err, http.StatusBadRequest)
			return
		}
		msg, err := pear.HandleNew(s)
		if err != nil {
			errorWrapper(w, err, http.StatusInternalServerError)
			return
		}
		if err = WriteMsg(w, msg); err != nil {
			errorWrapper(w, err, http.StatusInternalServerError)
			return
		}
		return
	}
}

func ExtractInteraction(r *http.Request) (*slack.InteractionCallback, error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	vals, err := url.ParseQuery(string(b))
	if err != nil {
		return nil, err
	}
	payload, ok := vals["payload"]
	if !ok {
		return nil, errors.New("missing payload")
	}
	var interaction slack.InteractionCallback
	if err = json.Unmarshal([]byte(payload[0]), &interaction); err != nil {
		return nil, fmt.Errorf("unable to unmarshal interaction: %w", err)
	}
	return &interaction, nil
}

func HandleSubmit(pear *PearService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interaction, err := ExtractInteraction(r)
		if err != nil {
			errorWrapper(w, err, http.StatusBadRequest)
			return
		}
		err = pear.HandleSubmit(interaction)
		if err != nil {
			errorWrapper(w, err, http.StatusInternalServerError)
			return
		}
		return
	}
}

func errorWrapper(w http.ResponseWriter, err error, code int) {
	hclog.Default().Error(err.Error())
	http.Error(w, err.Error(), code)
	return
}
