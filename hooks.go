package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type HookFn func(r *Run) error

type HookRunner struct {
	Fn map[string]HookFn
}

func NewHookRunner() HookRunner {
	return HookRunner{
		Fn: make(map[string]HookFn),
	}
}

// Hook returns the main hook for HookRunner, which runs all its hooks sequentially.
// In case of error it returns the first error and does not consider any other hooks.
func (h *HookRunner) Hook() HookFn {
	return func(r *Run) error {
		for name, hook := range h.Fn {
			err := hook(r)
			if err != nil {
				return fmt.Errorf("error running hook %q for run %s: %v", name, r.ID, err)
			}
		}
		return nil
	}
}

var GrafanaAnnotation = func(grafanaHost, grafanaAPIKey string) func(r *Run) error {
	type grafanaAnnotationPayload struct {
		Time int64    `json:"time"`
		Tags []string `json:"tags"`
		Text string   `json:"text"`
	}

	return func(r *Run) error {
		if len(r.Notifications) != 1 {
			return nil
		}

		payload := grafanaAnnotationPayload{
			Time: r.Notifications[0].RunUpdatedAt.UnixMilli(),
			Text: fmt.Sprintf("%s workspace %q: %q", r.Notifications[0].RunStatus, r.WorkspaceName, r.Message),
			Tags: []string{
				"tfc-badge",
				fmt.Sprintf("status:%s", r.Notifications[0].RunStatus),
				fmt.Sprintf("workspace:%s", r.WorkspaceName),
			},
		}

		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error encoding JSON payload: %w", err)
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/annotations", grafanaHost), bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("error creating new HTTP request: %w", err)
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", grafanaAPIKey))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("error making HTTP request to %q: %w", req.URL, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("http request unsucessful: %s", res.Status)
		}
		return nil
	}
}
