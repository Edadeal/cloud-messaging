package cms

import (
	"github.com/kikinteractive/go-gcm"
)

// GCM represent Google Cloud Messaging service client.
type GCM struct {
	errs       chan<- Response
	errsOpened bool
	client     gcm.Client
}

// NewGCMClient contructs an instance of GCM client.
func NewGCMClient(errs chan<- Response, apiKey, senderId string) (*GCM, error) {
	config := &gcm.Config{
		APIKey:   apiKey,
		SenderID: senderId,
	}

	cli, err := gcm.NewClient(config, func(cm gcm.CCSMessage) error {
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &GCM{errs: errs, errsOpened: true, client: cli}, nil
}

// Close closes client.
func (g *GCM) Close() error {
	if g.errsOpened {
		close(g.errs)
		g.errsOpened = false
	}
	return g.client.Close()
}

// NotifyOne notifies a recipient.
func (g *GCM) NotifyOne(message *Message, rec *Recipient) error {
	ttl := uint(86400)
	msg := gcm.HTTPMessage{
		To:         rec.Token,
		TimeToLive: &ttl,
		Priority:   "high",
		Data: map[string]interface{}{
			"title":       message.Title,
			"deeplink":    message.Deeplink,
			"body":        message.Body,
			"uuid":        "00000000-0000-0000-0000-000000000000",
			"mp_title":    message.Title,
			"mp_message":  message.Body,
			"mp_deeplink": message.Deeplink,
		},
		Notification: &gcm.Notification{
			Title: message.Title,
			Body:  message.Body,
		},
	}

	res, err := g.client.SendHTTP(msg)

	if err != nil {
		return err
	}

	g.errs <- Response{
		Ok:         res.Success == 200,
		StatusCode: res.StatusCode,
		Token:      rec.Token,
		Reason:     res.Results[0].Error,
		Id:         rec.Id,
	}

	return nil
}

// NotifyMany notifies a chunk of recipients with the same message.
func (g *GCM) NotifyMany(message *Message, recs []Recipient) error {
	list := make([]string, len(recs))

	for i, rec := range recs {
		list[i] = rec.Token
	}

	ttl := uint(86400)
	msg := gcm.HTTPMessage{
		RegistrationIDs: list,
		TimeToLive:      &ttl,
		Priority:        "high",
		Data: map[string]interface{}{
			"title":       message.Title,
			"deeplink":    message.Deeplink,
			"body":        message.Body,
			"uuid":        "00000000-0000-0000-0000-000000000000",
			"mp_title":    message.Title,
			"mp_message":  message.Body,
			"mp_deeplink": message.Deeplink,
		},
		Notification: &gcm.Notification{
			Title: message.Title,
			Body:  message.Body,
		},
	}

	res, err := g.client.SendHTTP(msg)

	if err != nil {
		return err
	}

	for i, r := range res.Results {
		g.errs <- Response{
			Id:         recs[i].Id,
			Ok:         len(r.Error) == 0,
			StatusCode: res.StatusCode,
			Reason:     r.Error,
			Token:      recs[i].Token,
		}
	}

	return nil
}
