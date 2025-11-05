package app

import (
	"errors"
	"fmt"
)

const (
	ColumnCapacity = 5
	CategoryLimit  = 5

	LocationCategory      = "category"
	LocationBackburner    = "backburner"
	LocationArchive       = "archive"
	LocationCategoryBoard = "board"
)

// BoardState represents the persisted board.
type BoardState struct {
	Categories         []Category `json:"categories"`
	Backburner         []Task     `json:"backburner"`
	Archives           []Task     `json:"archives"`
	CategoryBackburner []Category `json:"categoryBackburner"`
	CategoryArchives   []Category `json:"categoryArchives"`
}

type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

type Task struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Notes       string     `json:"notes"`
    State       string     `json:"state"`
    Size        int        `json:"size"`
    Links       []TaskLink `json:"links,omitempty"`
    Checklist   []ChecklistItem `json:"checklist,omitempty"`
    Urgent      bool       `json:"urgent,omitempty"`
    Focused     bool       `json:"focused,omitempty"`
    SourceID    string     `json:"sourceId,omitempty"`
    Source      string     `json:"source,omitempty"`
}

type TaskLink struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}

type ChecklistItem struct {
    Text string `json:"text"`
    Done bool   `json:"done"`
}

// Validation Errors
var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrCategoryNotFound  = errors.New("category not found")
	ErrCapacityExceeded  = errors.New("column capacity exceeded")
	ErrInvalidState      = errors.New("invalid state value")
	ErrInvalidLocation   = errors.New("invalid location")
	ErrInvalidTaskSize   = errors.New("task size must be between 1 and 5")
	ErrInvalidRequest    = errors.New("invalid request")
	ErrDuplicateCategory = errors.New("duplicate category name")
	ErrCategoryLimit     = errors.New("maximum number of categories reached")
)

func (t Task) Clone() Task {
    out := t
    if len(t.Links) > 0 {
        out.Links = make([]TaskLink, len(t.Links))
        copy(out.Links, t.Links)
    }
    if len(t.Checklist) > 0 {
        out.Checklist = make([]ChecklistItem, len(t.Checklist))
        copy(out.Checklist, t.Checklist)
    }
    return out
}

func (c Category) Clone() Category {
	out := c
	if len(c.Tasks) > 0 {
		out.Tasks = make([]Task, len(c.Tasks))
		for i := range c.Tasks {
			out.Tasks[i] = c.Tasks[i].Clone()
		}
	}
	return out
}

func (b BoardState) Clone() BoardState {
	out := BoardState{}
	if len(b.Categories) > 0 {
		out.Categories = make([]Category, len(b.Categories))
		for i := range b.Categories {
			out.Categories[i] = b.Categories[i].Clone()
		}
	}
	if len(b.Backburner) > 0 {
		out.Backburner = make([]Task, len(b.Backburner))
		for i := range b.Backburner {
			out.Backburner[i] = b.Backburner[i].Clone()
		}
	}
	if len(b.Archives) > 0 {
		out.Archives = make([]Task, len(b.Archives))
		for i := range b.Archives {
			out.Archives[i] = b.Archives[i].Clone()
		}
	}
	if len(b.CategoryBackburner) > 0 {
		out.CategoryBackburner = make([]Category, len(b.CategoryBackburner))
		for i := range b.CategoryBackburner {
			out.CategoryBackburner[i] = b.CategoryBackburner[i].Clone()
		}
	}
	if len(b.CategoryArchives) > 0 {
		out.CategoryArchives = make([]Category, len(b.CategoryArchives))
		for i := range b.CategoryArchives {
			out.CategoryArchives[i] = b.CategoryArchives[i].Clone()
		}
	}
	return out
}

var allowedStates = map[string]struct{}{
    "todo":      {},
    "doing":     {},
    "blocked":   {},
    "done":      {},
    "delegated": {},
}

func ValidateTaskState(state string) error {
	if _, ok := allowedStates[state]; !ok {
		return fmt.Errorf("%w: %s", ErrInvalidState, state)
	}
	return nil
}

func NormalizeSize(size int) (int, error) {
	if size < 1 || size > 5 {
		return 0, ErrInvalidTaskSize
	}
	return size, nil
}
