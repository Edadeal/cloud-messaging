package cms

import (
	"github.com/NaySoftware/go-fcm"
	"log"
)

// FCM represent Firebase Cloud Messaging service client.
type FCM struct {
	errs       chan<- Response
	errsOpened bool
	apiKey     string
	client     *fcm.FcmClient
}

// NewFCMClient contructs an instance of FCM client.
func NewFCMClient(errs chan<- Response, apiKey string) (*FCM, error) {
	client := fcm.NewFcmClient(apiKey)

	return &FCM{
		errs:       errs,
		errsOpened: true,
		apiKey:     apiKey,
		client:     client,
	}, nil
}

// Close closes client.
func (f *FCM) Close() error {
	if f.errsOpened {
		close(f.errs)
		f.errsOpened = false
	}
	return nil
}

// NotifyOne notifies a recipient.
func (f *FCM) NotifyOne(message *Message, rec *Recipient) error {
	body := &fcm.NotificationPayload{
		Title: message.Title,
		Body:  message.Body,
	}

	res, err := f.client.
		NewFcmMsgTo(rec.Token, body).
		SetMsgData(map[string]interface{}{
			"title":       message.Title,
			"body":        message.Body,
			"deeplink":    message.Deeplink,
			"uuid":        "00000000-0000-0000-0000-000000000000",
			"mp_title":    message.Title,
			"mp_message":  message.Body,
			"mp_deeplink": message.Deeplink,
		}).Send()

	reason := ""

	if len(res.Results) > 0 {
		reason = res.Results[0]["error"]
	}

	f.errs <- Response{
		Id:         rec.Id,
		Ok:         res.StatusCode == 200,
		Reason:     reason,
		StatusCode: res.StatusCode,
		Token:      rec.Token,
	}

	return err
}

// NotifyMany notifies a chunk of recipients with the same message.
func (f *FCM) NotifyMany(message *Message, recs []Recipient) error {
	list := make([]string, len(recs))

	for i, rec := range recs {
		list[i] = rec.Token
	}

	body := &fcm.NotificationPayload{
		Title: message.Title,
		Body:  message.Body,
	}

	send := func(f *FCM) (*fcm.FcmResponseStatus, error) {
		return f.client.
			NewFcmRegIdsMsg(list, body).
			SetMsgData(map[string]interface{}{
				"title":       message.Title,
				"body":        message.Body,
				"deeplink":    message.Deeplink,
				"uuid":        "00000000-0000-0000-0000-000000000000",
				"mp_title":    message.Title,
				"mp_message":  message.Body,
				"mp_deeplink": message.Deeplink,
			}).Send()
	}

	for i := 0; i != 3; i += 1 {
		res, err := send(f)

		if err != nil && i == 2 {
			return err
		} else if err != nil {
			log.Printf("fcm failed: %s", err.Error())
			log.Printf("retry again #%d", i)
			f.client = fcm.NewFcmClient(f.apiKey)
			continue
		}

		log.Printf("FCM response status is %t", res.Ok)
		log.Printf("FCM retry after %s", res.RetryAfter)
		log.Printf("FCM number of success sends is %d", res.Success)
		log.Printf("FCM number of failures is %d", res.Fail)
		log.Printf("FCM error is %s", res.Err)
		log.Printf("FCM status code is %d", res.StatusCode)
		log.Printf("number of recipients is %d", len(recs))
		log.Printf("number of response results is %d", len(res.Results))

		for i, rec := range recs {
			f.errs <- Response{
				Id:         rec.Id,
				Ok:         res.StatusCode == 200,
				Reason:     res.Results[i]["error"],
				StatusCode: res.StatusCode,
				Token:      rec.Token,
			}
		}

		break
	}

	return nil
}
