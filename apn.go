package cms

import (
	"crypto/tls"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
)

// APN represent Apple Notification Service client.
type APN struct {
	errs       chan<- Response
	errsOpened bool
	client     *apns2.Client
	cert       tls.Certificate
	topic      string
}

// NewAPNClient contructs an instance of APNS client.
func NewAPNClient(errs chan<- Response, certPath, topic string) (*APN, error) {
	pass := ""
	cert, err := certificate.FromP12File(certPath, pass)

	if err != nil {
		return nil, err
	}

	client := apns2.NewClient(cert).Production()

	return &APN{
		errs:       errs,
		errsOpened: true,
		client:     client,
		cert:       cert,
		topic:      topic,
	}, nil
}

// Close closes client.
func (a *APN) Close() error {
	if a.errsOpened {
		close(a.errs)
		a.errsOpened = false
	}
	return nil
}

// NotifyOne notifies a recipient.
func (a *APN) NotifyOne(message *Message, rec *Recipient) error {
	payload := payload.NewPayload().
		AlertTitle(message.Title).
		AlertBody(message.Body).
		Custom("deeplink", message.Deeplink)

	notification := &apns2.Notification{}
	notification.DeviceToken = rec.Token
	notification.Topic = a.topic
	notification.Payload = payload

	res, err := a.client.Push(notification)

	if err != nil {
		return err
	}

	a.errs <- Response{
		Id:         rec.Id,
		Ok:         res.StatusCode == 200,
		Reason:     res.Reason,
		StatusCode: res.StatusCode,
		Token:      rec.Token,
	}

	return nil
}

// NotifyMany notifies a chunk of recipients with the same message.
func (a *APN) NotifyMany(message *Message, recs []Recipient) error {
	var err error

	errs := make(chan error, len(recs))
	notify := func(rec *Recipient) {
		err := a.NotifyOne(message, rec)
		errs <- err
	}

	for _, el := range recs {
		go notify(&el)
	}

	for range recs {
		err = <-errs
	}

	return err
}
