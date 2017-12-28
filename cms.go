package cms

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"

	"github.com/satori/go.uuid"
)

// APNConig stores settings for APNS client.
type APNConfig struct {
	Cert  string
	Topic string
}

// FCMConfig stores settings for FCM client.
type FCMConfig struct {
	APIKey string `toml:"api_key"`
}

// GCMConfig stores settings for GCM client.
type GCMConfig struct {
	APIKey   string `toml:"api_key"`
	SenderID string `toml:"sender_id"`
}

// WNSConfig stores settings for WNS client.
type WNSConfig struct {
}

// CMSConfig collect settings for all supported messaging services.
type CMSConfig struct {
	APN APNConfig
	FCM FCMConfig
	GCM GCMConfig
	WNS WNSConfig
}

// Recipient identifies recipient by its UUID and push token string.
type Recipient struct {
	Id    uuid.UUID
	Token string
}

// Recipients represents object to iterates over lines of csv file
// with recipients identifiers like id or token.
type Recipients struct {
	csv        *csv.Reader
	indexId    int
	indexToken int
	noline     int
}

// RecipientsFromFile create instance of Recipients structure from csv file by
// its filename.
func RecipientsFromFile(path string) (*Recipients, error) {
	b, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	log.Println("load tokens from", path)
	reader := bytes.NewReader(b)
	csv := csv.NewReader(reader)

	var indexId, indexToken int

	if parts, err := csv.Read(); err != nil {
		return nil, err
	} else if len(parts) != 2 {
		return nil, errors.New("token files should be CSV file(id;token)")
	} else {
		if parts[0] == "id" && parts[1] == "token" {
			indexId = 0
			indexToken = 1
		} else if parts[1] == "id" && parts[0] == "token" {
			indexId = 1
			indexToken = 0
		} else {
			return nil, errors.New("token files should be CSV file(id;token)")
		}
	}

	return &Recipients{
		csv:        csv,
		indexId:    indexId,
		indexToken: indexToken,
		noline:     1,
	}, nil
}

// Next does step of iteration and returns next recipient.
func (r *Recipients) Next() (*Recipient, error) {
	parts, err := r.csv.Read()

	if err != nil {
		return nil, err
	} else if len(parts) != 2 {
		return nil, errors.New("too few columns in line")
	}

	r.noline++

	return &Recipient{
		Id:    uuid.FromStringOrNil(parts[r.indexId]),
		Token: parts[r.indexToken],
	}, nil
}

// Response contains status information of sent notification.
type Response struct {
	Id         uuid.UUID
	Ok         bool
	Reason     string
	StatusCode int
	Token      string
}

// Responses contains chunk of response for batch processing.
type Responses struct {
	Success   uint
	Failure   uint
	Responses []Response
}

// Message contains push notification payload.
type Message struct {
	Title    string
	Body     string
	Deeplink string
}

// MessageFromFile fills notification payload from json file.
func MessageFromFile(source string) (*Message, error) {
	bytes, err := ioutil.ReadFile(source)

	if err != nil {
		return nil, err
	}

	message := &Message{}

	if err := json.Unmarshal(bytes, message); err != nil {
		return nil, err
	}

	return message, nil
}

// CMS generalize notion of Cloud Message Service client like APNS or GCM.
type CMS interface {
	// Close closes CMS client.
	Close() error

	// NotifyOne notifies only one recipient.
	NotifyOne(*Message, *Recipient) error

	// NotifeMany notifies multiple recipients.
	NotifyMany(*Message, []Recipient) error
}

// Notify sends push notification to recipients that are recieved through the
// channel. It use given CMS as a client and the same message for all
// recipeints.
func Notify(c CMS, message *Message, pipe <-chan Recipient) error {
	recs := make([]Recipient, 512)

	for {
		index, is_closed := MakeBatch(pipe, recs)

		if err := c.NotifyMany(message, recs[:index]); err != nil {
			return err
		}

		if is_closed {
			return nil
		}
	}

	return nil
}

// MakeBatch groups recipients from a channel into chunk and puths them into
// slice. It returns size of batch and status of fetching from channel.
func MakeBatch(pipe <-chan Recipient, buffer []Recipient) (int, bool) {
	index := 0

	for index = 0; index != len(buffer); index++ {
		select {
		case rec, ok := <-pipe:
			if !ok && len(rec.Token) == 0 && uuid.Equal(rec.Id, uuid.Nil) {
				return index, true
			} else {
				buffer[index] = rec
			}
		}
	}

	return index, false
}

// CMSFactory contains common configuration for any CMS client and produce
// client by its name.
type CMSFactory struct {
	responses chan<- Response
	cfg       *CMSConfig
}

// NewCMSFactory creates new instance of factory.
func NewCMSFactory(cfg *CMSConfig, errs chan<- Response) (*CMSFactory, error) {
	return &CMSFactory{
		responses: errs,
		cfg:       cfg,
	}, nil
}

// Produce makes new instance of CMS client object by its name or target
// platform. Avaliable names are `apn` or `ios` for APNS, `fcm` or `android`
// for FCM, `gcm` for GCM and `wns` or `windows` for WNS.
func (f CMSFactory) Produce(name string, responses chan<- Response) (CMS, error) {
	if responses == nil && f.responses == nil {
		return nil, errors.New("response channel could not be nil")
	} else if responses == nil {
		responses = f.responses
	}

	switch name {
	case "apn":
		fallthrough
	case "ios":
		return NewAPNClient(responses, f.cfg.APN.Cert, f.cfg.APN.Topic)
	case "fcm":
		fallthrough
	case "android":
		return NewFCMClient(responses, f.cfg.FCM.APIKey)
	case "gcm":
		return NewGCMClient(responses, f.cfg.GCM.APIKey, f.cfg.GCM.SenderID)
	case "wns":
		fallthrough
	case "windows":
		return nil, errors.New("not implemented")
	default:
		return nil, errors.New("unknown CMS")
	}
}
