package lobby

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Placement struct{ ID, Game, Name, Host string }

type PlacementStore interface {
	Assign(context.Context, Placement) (string, error)
}

var ErrPlacementConflict = errors.New("This server ID is already in use")

// Assignment survives Lambda retries and remains with the server while it is stopped.
func (t *Table) Assign(ctx context.Context, p Placement) (string, error) {
	item := map[string]types.AttributeValue{"id": textValue("placement/" + p.ID), "game": textValue(p.Game), "name": textValue(p.Name), "host": textValue(p.Host)}
	_, err := t.Client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(t.Name), Item: item, ConditionExpression: aws.String("attribute_not_exists(id)")})
	if err == nil {
		return p.Host, nil
	}
	var conflict *types.ConditionalCheckFailedException
	if !errors.As(err, &conflict) {
		return "", err
	}
	old, err := t.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(t.Name), Key: map[string]types.AttributeValue{"id": item["id"]}, ConsistentRead: aws.Bool(true)})
	if err != nil {
		return "", err
	}
	if textAttr(old.Item, "game") != p.Game || textAttr(old.Item, "name") != p.Name {
		return "", ErrPlacementConflict
	}
	owner := textAttr(old.Item, "host")
	if owner == "" {
		return "", fmt.Errorf("server assignment has no host")
	}
	return owner, nil
}

func (f *Fleet) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID   string `json:"id"`
		Game string `json:"game"`
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond(w, 400, map[string]string{"error": "Invalid server request"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		respond(w, 400, map[string]string{"error": "Invalid server request"})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = input.Game
	}
	if !instanceID.MatchString(input.ID) || !strings.HasPrefix(input.ID, "server-") || len(input.ID) != 39 || len(input.Name) > 80 || len(input.Game) > 80 || input.Game == "" {
		respond(w, 400, map[string]string{"error": "Choose a game and a server name"})
		return
	}
	type candidate struct {
		member     Member
		template   string
		load, rank int
	}
	candidates := []candidate{}
	existingOwner := ""
	for _, m := range f.Members {
		row, err := m.Service.Table.Read(r.Context(), input.ID)
		if err != nil {
			m.Service.fail(w, err)
			return
		}
		if row.Snapshot.ID != "" {
			if row.Snapshot.Game != input.Game || row.Snapshot.Name != input.Name || row.Snapshot.Desired == "deleted" || existingOwner != "" {
				respond(w, 409, map[string]string{"error": ErrPlacementConflict.Error()})
				return
			}
			existingOwner = m.ID
		}
		listing, err := m.Service.view(r.Context())
		if err != nil {
			log.Printf("choose host %s: %v", m.ID, err)
			continue
		}
		load := 0
		for _, g := range listing.Games {
			if g.Desired == "running" || g.Pending && g.Action == "start" {
				load++
			}
		}
		rank := 0
		switch listing.Host {
		case "running":
		case "pending":
			rank = 1
		case "stopped", "stopping":
			rank = 2
		default:
			continue
		}
		for _, p := range listing.Presets {
			if p["game"] == input.Game {
				candidates = append(candidates, candidate{member: m, template: p["id"], load: load, rank: rank})
				break
			}
		}
	}
	if len(candidates) == 0 {
		respond(w, 503, map[string]string{"error": "No host is available for this game. Try again shortly."})
		return
	}
	selected := candidates[0]
	for _, c := range candidates {
		score := sha256.Sum256([]byte(input.ID + "/" + c.member.ID))
		best := sha256.Sum256([]byte(input.ID + "/" + selected.member.ID))
		if c.rank < selected.rank || c.rank == selected.rank && (c.load < selected.load || c.load == selected.load && bytes.Compare(score[:], best[:]) < 0) {
			selected = c
		}
	}
	if existingOwner != "" {
		found := false
		for _, c := range candidates {
			if c.member.ID == existingOwner {
				selected = c
				found = true
				break
			}
		}
		if !found {
			respond(w, 503, map[string]string{"error": "This server's host is unavailable. Try again shortly."})
			return
		}
	}
	if f.Placements == nil {
		respond(w, 503, map[string]string{"error": "Server placement is unavailable"})
		return
	}
	owner, err := f.Placements.Assign(r.Context(), Placement{ID: input.ID, Game: input.Game, Name: input.Name, Host: selected.member.ID})
	if err != nil {
		status := 503
		message := "Couldn't save this server request. Try again."
		if errors.Is(err, ErrPlacementConflict) {
			status = 409
			message = ErrPlacementConflict.Error()
		} else {
			log.Printf("assign server %s: %v", input.ID, err)
		}
		respond(w, status, map[string]string{"error": message})
		return
	}
	for _, c := range candidates {
		if c.member.ID != owner {
			continue
		}
		payload, _ := json.Marshal(map[string]string{"id": input.ID, "template": c.template, "name": input.Name})
		request := r.Clone(r.Context())
		request.URL.Path = "/api/games"
		request.Body = io.NopCloser(bytes.NewReader(payload))
		request.ContentLength = int64(len(payload))
		c.member.Service.routes().ServeHTTP(w, request)
		return
	}
	respond(w, 503, map[string]string{"error": "This server's host is unavailable. Try again shortly."})
}
