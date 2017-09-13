package trac

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const timeFormat = "2006-01-02T15:04:05"

type TicketField struct {
	Label    string
	Name     string
	Options  []string
	Type     string
	Value    string
	Format   string
	Order    int
	Custom   bool
	Optional bool
}

type Ticket struct {
	client *Client

	Id          int       `json:"id"`
	Time        time.Time `json:"time"`
	Changetime  time.Time `json:"changetime"`
	Owner       string    `json:"owner,omitempty"`
	Reporter    string    `json:"reporter"`
	Summary     string    `json:"summary,omitempty"`
	Description string    `json:"decription,omitempty"`
	Project     string    `json:"project,omitempty"`
	Status      string    `json:"status,omitempty"`
	Type        string    `json:"type,omitempty"`
	Priority    string    `json:"priority,omitempty"`
	Milestone   string    `json:"milestone,omitempty"`
	Component   string    `json:"component,omitempty"`
	BlockedBy   string    `json:"blockedby,omitempty"`
	Blocking    string    `json:"blocking,omitempty"`
	Keywords    string    `json:"keywords,omitempty"`
	Parents     string    `json:"parents,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
	Version     string    `json:"version,omitempty"`
}

type Component struct {
	Description string
	Name        string
	Owner       string
}

type Milestone struct {
	Name        string
	Description string
	Due         int
	Completed   int
}

type Version struct {
	Name        string
	Description string
	Time        time.Time
}

func (v *Version) UnmarshalJSON(in []byte) error {
	type Alias Version
	tmp := struct {
		*Alias
		Time CustomType
	}{
		Alias: (*Alias)(v),
	}
	if err := json.Unmarshal(in, &tmp); err != nil {
		return err
	}
	t, err := time.Parse(timeFormat, tmp.Time.Kv[1])
	if err != nil {
		return err
	}
	v.Time = t
	return nil
}

func (t *Ticket) setField(field string, value string) bool {
	f := reflect.ValueOf(t).Elem().FieldByName(field)
	if f.IsValid() && f.CanAddr() {
		f.SetString(value)
		return true
	}
	return false
}

func (t *Ticket) setTime(field, value string) bool {
	f := reflect.ValueOf(t).Elem().FieldByName(field)
	if f.IsValid() && f.CanAddr() {
		t, _ := time.Parse(timeFormat, value)
		f.Set(reflect.ValueOf(t))
		return true
	}
	return false
}

// setTimes is a convenience method to avoid nesting.
func (t *Ticket) setTimes(field string, values map[string]interface{}) {

	for _, iiii := range values {
		switch vvvv := iiii.(type) {
		case []interface{}:
			for _, tt := range vvvv {
				if tt != "datetime" {
					t.setTime(field, fmt.Sprintf("%s", tt))
				}
			}
		}
	}
}

func (t *Ticket) UnmarshalJSON(in []byte) error {
	var data []interface{}
	if err := json.Unmarshal(in, &data); err != nil {
		return err
	}

	for _, i := range data {
		switch v := i.(type) {
		case float64:
			t.Id = int(v)
		case map[string]interface{}:
			for kk, ii := range v {
				kkt := strings.Title(kk)
				switch vv := ii.(type) {
				case string:
					t.setField(kkt, vv)
				case map[string]interface{}:
					t.setTimes(kkt, vv)
				}
			}
		}
	}
	return nil
}

// GetIds returns all open tickets IDs.
func (t *Ticket) GetIds() ([]int, error) {
	r, err := t.client.Query("ticket.query", "max=0&status!=closed")
	if err != nil {
		return nil, err
	}
	var ids []int
	if err := json.Unmarshal(r.Result, &ids); err != nil {
		return nil, err
	}

	return ids, nil
}

// Get returns a ticket by its number.
func (t *Ticket) Get(number int) (Ticket, error) {
	var tkt = Ticket{}
	r, err := t.client.Query("ticket.get", strconv.Itoa(number))
	if err != nil {
		return tkt, err
	}

	if err := json.Unmarshal(r.Result, &tkt); err != nil {
		return tkt, err
	}
	return tkt, nil
}

type Attachment struct {
	Filename    string
	Description string
	Size        int
	Time        time.Time
	Author      string
	Binary      string // base64 encoded
}

func (a *Attachment) UnmarshalJSON(in []byte) error {
	data := []interface{}{
		&a.Filename,
		&a.Description,
		&a.Size,
		"",
		&a.Author,
	}
	if err := json.Unmarshal(in, &data); err != nil {
		return err
	}

	d, ok := data[3].(map[string]interface{})
	if !ok {
		return errors.New("Can't decode attachment date")
	}

	for _, i := range d {
		switch v := i.(type) {
		case []interface{}:
			for _, tt := range v {
				if tt != "datetime" {
					t, _ := time.Parse(timeFormat, tt.(string))
					a.Time = t
				}
			}
		}
	}

	return nil
}

// Attachments returns attachments metadata for a given ticket number.
func (t *Ticket) Attachments(ticket int) ([]Attachment, error) {
	var attch []Attachment
	_, err := t.client.Do("ticket.listAttachments", &attch, strconv.Itoa(ticket))
	return attch, err
}

// Attachment returns the attachment binary.
func (t *Ticket) Attachment(ticket int, name string) ([]byte, error) {
	var out *[]byte
	r, err := t.client.Query("ticket.getAttachment", strconv.Itoa(ticket), name)
	if err != nil {
		return *out, err
	}

	var response map[string]interface{}

	if err := json.Unmarshal(r.Result, &response); err != nil {
		return *out, err
	}

	for _, i := range response {
		switch v := i.(type) {
		case []interface{}:
			for _, ii := range v {
				if ii != "binary" {
					b, err := base64.StdEncoding.DecodeString(ii.(string))
					out = &b
					if err != nil {
						return *out, err
					}
				}
			}
		default:
			fmt.Printf("%v\n", v.(string))
		}
	}

	return *out, nil
}

// AddAttachment is not implemented.
func (t *Ticket) AddAttachment(ticket int) (string, error) {
	return "", fmt.Errorf("Not implemented")
}

// DelAttachment deletes an attachment.
func (t *Ticket) DelAttachment(ticket int, attachment string) (bool, error) {
	var r bool
	_, err := t.client.Do(
		"ticket.deleteAttachment", &r, strconv.Itoa(ticket), attachment,
	)
	return r, err
}

// Fields returns a list of all ticket fields.
func (t *Ticket) Fields() ([]TicketField, error) {
	var f = []TicketField{}
	_, err := t.client.Do("ticket.getTicketFields", &f)
	return f, err
}

// Query performs a ticket query, returning a list of ticket ID's. All queries
// will use stored settings for maximum number of results per page and paging
// options.
func (t *Ticket) Query(str string) ([]int, error) {
	var r []int
	_, err := t.client.Do("ticket.query", &r, str)
	return r, err
}

// RecentChanges is not implemented.
func (t *Ticket) RecentChanges(since time.Time) ([]int, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Actions is not implemented.
func (t *Ticket) Actions(ticket int) ([]string, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Create is not implemented.
func (t *Ticket) Create() error {
	return fmt.Errorf("Not implemented")
}

// Update is not implemented.
func (t *Ticket) Update(ticket int) ([]string, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Delete ticket withe the given ticket id.
func (t *Ticket) Delete(ticket int) (int, error) {
	var r int
	_, err := t.client.Do("ticket.delete", &r, strconv.Itoa(ticket))
	return r, err
}

// Changelog is not implemented.
func (t *Ticket) Changelog(ticket int) error {
	return fmt.Errorf("Not implemented")
}

// Components returns a list of all ticket components names.
func (t *Ticket) Components() ([]string, error) {
	return t.client.All("ticket.component.getAll")
}

// GetComponent returns a component by its `name`.
func (t *Ticket) GetComponent(name string) (Component, error) {
	var c Component
	_, err := t.client.Do("ticket.component.get", &c, name)
	return c, err
}

// DelComponent deletes a component by name.
func (t *Ticket) DelComponent(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.component.delete", &r, name)
	return r, err
}

func (t *Ticket) AddComponent(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

func (t *Ticket) SetComponent(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Milestones returns a list of all ticket milestones names.
func (t *Ticket) Milestones() ([]string, error) {
	return t.client.All("ticket.milestone.getAll")
}

// MilestoneId returns the ID of the milestone `name`.
func (t *Ticket) MilestoneId(name string) (Milestone, error) {
	var m Milestone
	_, err := t.client.Do("ticket.milestone.get", &m, name)
	return m, err
}

// DelMilestone deletes a milestone by name.
func (t *Ticket) DelMilestone(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.milestone.delete", &r, name)
	return r, err
}

func (t *Ticket) AddMilestone(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

func (t *Ticket) SetMilestone(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Priorities returns a list of all ticket priority names.
func (t *Ticket) Priorities() ([]string, error) {
	return t.client.All("ticket.priority.getAll")
}

// PriorityId returns the ID of the priority `name`.
func (t *Ticket) PriorityId(name string) (int, error) {
	var p string
	_, err := t.client.Do("ticket.priority.get", &p, name)
	i, err := strconv.Atoi(p)
	if err != nil {
		return i, err
	}
	return i, err
}

// AddPriority is not implemented.
func (t *Ticket) AddPriority(name, value string) error {
	return fmt.Errorf("Not implemented")
}

// DelPriority deletes a priority by name.
func (t *Ticket) DelPriority(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.priority.delete", &r, name)
	return r, err
}

// SetPriority is not implemented.
func (t *Ticket) SetPriority(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Resolutions returns a list of all ticket resolution names.
func (t *Ticket) Resolutions() ([]string, error) {
	return t.client.All("ticket.resolution.getAll")
}

// ResolutionId returns the ID of the resolution `name`.
func (t *Ticket) ResolutionId(name string) (int, error) {
	var r string
	_, err := t.client.Do("ticket.resolution.get", &r, name)
	i, err := strconv.Atoi(r)
	if err != nil {
		return i, err
	}
	return i, err
}

func (t *Ticket) AddResolution(name, value string) error {
	return fmt.Errorf("Not implemented")
}

// DelResolution deletes a resolution by name.
func (t *Ticket) DelResolution(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.resolution.delete", &r, name)
	return r, err
}

// SetResolution is not implemented.
func (t *Ticket) SetResolution(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Severities returns a list of all ticket severity names.
func (t *Ticket) Severities() ([]string, error) {
	return t.client.All("ticket.severity.getAll")
}

// Severity returns the ID of the severity `name`.
func (t *Ticket) SeverityId(name string) (int, error) {
	var s string
	_, err := t.client.Do("ticket.severity.get", &s, name)
	i, err := strconv.Atoi(s)
	if err != nil {
		return i, err
	}
	return i, err
}

// AddSeverity is not implemented.
func (t *Ticket) AddSeverity(name, value string) error {
	return fmt.Errorf("Not implemented")
}

// DelSeverity deletes a severity by name.
func (t *Ticket) DelSeverity(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.severity.delete", &r, name)
	return r, err
}

// SetSeverity is not implemented.
func (t *Ticket) SetSeverity(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Statuses returns all ticket states described by active workflow.
func (t *Ticket) Statuses() ([]string, error) {
	return t.client.All("ticket.status.getAll")
}

// Types returns a list of all ticket type names.
func (t *Ticket) Types() ([]string, error) {
	return t.client.All("ticket.type.getAll")
}

// TypeId returns the ID of the type `name`.
func (t *Ticket) TypeId(name string) (int, error) {
	var s string
	_, err := t.client.Do("ticket.type.get", &s, name)
	i, err := strconv.Atoi(s)
	if err != nil {
		return i, err
	}
	return i, err
}

// AddType is not implemented.
func (t *Ticket) AddType(name, value string) error {
	return fmt.Errorf("Not implemented")
}

// DelType deletes a type by name.
func (t *Ticket) DelType(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.type.delete", &r, name)
	return r, err
}

// SetType is not implemented.
func (t *Ticket) SetType(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// Versions returns a list of all ticket version names.
func (t *Ticket) Versions() ([]string, error) {
	return t.client.All("ticket.version.getAll")
}

// GetVersion returns version information.
func (t *Ticket) GetVersion(name string) (Version, error) {
	var v Version
	_, err := t.client.Do("ticket.version.get", &v, name)
	return v, err
}

// DelVersion deletes a version by name.
func (t *Ticket) DelVersion(name string) (int, error) {
	var r int
	_, err := t.client.Do("ticket.version.delete", &r, name)
	return r, err
}

// AddVersion is not implemented.
func (t *Ticket) AddVersion(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}

// SetVersion is not implemented.
func (t *Ticket) SetVersion(name string, attrs []string) error {
	return fmt.Errorf("Not implemented")
}