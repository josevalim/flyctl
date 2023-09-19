package flypkgs

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Release struct {
	ID          uint64    `json:"id"`
	Channel     string    `json:"channel"`
	Version     string    `json:"version"`
	GitCommit   string    `json:"git_commit"`
	GitBranch   string    `json:"git_branch"`
	GitRef      string    `json:"git_ref"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	InsertedAt  time.Time `json:"inserted_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PublishedAt time.Time `json:"published_at"`
}

func (c *Client) GetReleaseByVersion(ctx context.Context, version string) (*Release, error) {
	req, err := http.NewRequest("GET", c.URL("/releases/version/%s", version), nil)
	if err != nil {
		return nil, err
	}

	res := Release{}
	if err := c.sendRequest(ctx, req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Client) UploadRelease(ctx context.Context, version string, r io.Reader) (*Release, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "release.tar.gz")
	if err != nil {
		// fmt.Println(err)
		return nil, err
	}
	_, err = io.Copy(part, r)
	if err != nil {
		// fmt.Println(err)
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		// fmt.Println(err)
		return nil, err
	}

	req, err := http.NewRequest("POST", c.URL("/releases/%s", version), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json; charset=utf-8")

	var out Release
	if err := c.sendRequest(ctx, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}
