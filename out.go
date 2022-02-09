package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Put (business logic)
func Put(request PutRequest, manager Github, inputDir string) (*PutResponse, error) {
	if err := request.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parameters: %s", err)
	}
	path := filepath.Join(inputDir, request.Params.Path, ".git", "resource")

	// Version available after a GET step.
	var version Version
	content, err := ioutil.ReadFile(filepath.Join(path, "version.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read version from path: %s", err)
	}
	if err := json.Unmarshal(content, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version from file: %s", err)
	}

	// Metadata available after a GET step.
	var metadata Metadata
	content, err = ioutil.ReadFile(filepath.Join(path, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from path: %s", err)
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata from file: %s", err)
	}

	// Set status if specified
	if p := request.Params; p.Status != "" || p.StatusFile != "" {
		status := p.Status
		description := p.Description

		// Set status from a file
		if p.StatusFile != "" {
			content, err := ioutil.ReadFile(filepath.Join(inputDir, p.StatusFile))
			if err != nil {
				return nil, fmt.Errorf("failed to read status file: %s", err)
			}
			status = strings.TrimSpace(string(content))
			err = validateStatus(status)
			if err != nil {
				return nil, err
			}
		}

		// Set description from a file
		if p.DescriptionFile != "" {
			content, err := ioutil.ReadFile(filepath.Join(inputDir, p.DescriptionFile))
			if err != nil {
				return nil, fmt.Errorf("failed to read description file: %s", err)
			}
			description = string(content)
		}

		if err := manager.UpdateCommitStatus(version.Commit, p.BaseContext, safeExpandEnv(p.Context), status, safeExpandEnv(p.TargetURL), description); err != nil {
			return nil, fmt.Errorf("failed to set status: %s", err)
		}
	}

	// Delete previous comments if specified
	if request.Params.DeletePreviousComments {
		err = manager.DeletePreviousComments(version.PR)
		if err != nil {
			return nil, fmt.Errorf("failed to delete previous comments: %s", err)
		}
	}

	// Set comment if specified
	if p := request.Params; p.Comment != "" {
		err = manager.PostComment(version.PR, safeExpandEnv(p.Comment))
		if err != nil {
			return nil, fmt.Errorf("failed to post comment: %s", err)
		}
	}

	// Set comment from a file
	if p := request.Params; p.CommentFile != "" {
		content, err := ioutil.ReadFile(filepath.Join(inputDir, p.CommentFile))
		if err != nil {
			return nil, fmt.Errorf("failed to read comment file: %s", err)
		}
		comment := string(content)
		if comment != "" {
			err = manager.PostComment(version.PR, safeExpandEnv(comment))
			if err != nil {
				return nil, fmt.Errorf("failed to post comment: %s", err)
			}
		}
	}

	return &PutResponse{
		Version:  version,
		Metadata: metadata,
	}, nil
}

// PutRequest ...
type PutRequest struct {
	Source Source        `json:"source"`
	Params PutParameters `json:"params"`
}

// PutResponse ...
type PutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata,omitempty"`
}

// PutParameters for the resource.
type PutParameters struct {
	Path                   string `json:"path"`
	BaseContext            string `json:"base_context"`
	Context                string `json:"context"`
	TargetURL              string `json:"target_url"`
	DescriptionFile        string `json:"description_file"`
	Description            string `json:"description"`
	StatusFile             string `json:"status_file"`
	Status                 string `json:"status"`
	CommentFile            string `json:"comment_file"`
	Comment                string `json:"comment"`
	DeletePreviousComments bool   `json:"delete_previous_comments"`
}

// Validate the put parameters.
func (p *PutParameters) Validate() error {
	if p.Status == "" {
		return nil
	}
	// Make sure we are setting an allowed status
	return validateStatus(p.Status)
}

func validateStatus(status string) error {
	var allowedStatus bool

	normalizedStatus := strings.ToLower(status)
	allowed := []string{"success", "pending", "failure", "error"}

	for _, a := range allowed {
		if normalizedStatus == a {
			allowedStatus = true
		}
	}

	if !allowedStatus {
		return fmt.Errorf("unknown status: %s", status)
	}
	return nil
}

func safeExpandEnv(s string) string {
	return os.Expand(s, func(v string) string {
		switch v {
		case "BUILD_ID", "BUILD_NAME", "BUILD_JOB_NAME", "BUILD_PIPELINE_NAME", "BUILD_TEAM_NAME", "ATC_EXTERNAL_URL":
			return os.Getenv(v)
		}
		return "$" + v
	})
}
