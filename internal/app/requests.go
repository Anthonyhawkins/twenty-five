package app

import "fmt"

type CreateTaskRequest struct {
	Location   string `json:"location"`
	CategoryID string `json:"categoryId,omitempty"`
	Position   *int   `json:"position,omitempty"`
	Task       Task   `json:"task"`
}

func (r *CreateTaskRequest) Normalize() {
	if r.Location == "" {
		r.Location = LocationCategory
	}
}

func (r CreateTaskRequest) Validate() error {
	if err := ValidateTaskState(r.Task.State); err != nil {
		return err
	}
	if _, err := NormalizeSize(r.Task.Size); err != nil {
		return err
	}
	switch r.Location {
	case LocationCategory:
		if r.CategoryID == "" {
			return fmt.Errorf("%w: categoryId required for category location", ErrInvalidRequest)
		}
	case LocationBackburner, LocationArchive:
	default:
		return ErrInvalidLocation
	}
	return nil
}

type TaskPatch struct {
    Name        *string     `json:"name,omitempty"`
    Description *string     `json:"description,omitempty"`
    Notes       *string     `json:"notes,omitempty"`
    State       *string     `json:"state,omitempty"`
    Size        *int        `json:"size,omitempty"`
    Links       *[]TaskLink `json:"links,omitempty"`
    Checklist   *[]ChecklistItem `json:"checklist,omitempty"`
    Urgent      *bool       `json:"urgent,omitempty"`
}

func (p TaskPatch) Apply(task *Task) error {
	if p.Name != nil {
		task.Name = *p.Name
	}
	if p.Description != nil {
		task.Description = *p.Description
	}
	if p.Notes != nil {
		task.Notes = *p.Notes
	}
	if p.State != nil {
		if err := ValidateTaskState(*p.State); err != nil {
			return err
		}
		task.State = *p.State
	}
	if p.Size != nil {
		size, err := NormalizeSize(*p.Size)
		if err != nil {
			return err
		}
		task.Size = size
	}
    if p.Links != nil {
        task.Links = make([]TaskLink, len(*p.Links))
        copy(task.Links, *p.Links)
    }
    if p.Checklist != nil {
        task.Checklist = make([]ChecklistItem, len(*p.Checklist))
        copy(task.Checklist, *p.Checklist)
    }
    if p.Urgent != nil {
        task.Urgent = *p.Urgent
    }
    return nil
}

type MoveTaskRequest struct {
	Location   string `json:"location"`
	CategoryID string `json:"categoryId,omitempty"`
	Position   *int   `json:"position,omitempty"`
	SourceID   string `json:"sourceId,omitempty"`
	Source     string `json:"source,omitempty"`
}

func (r *MoveTaskRequest) Normalize() {
	if r.Location == "" {
		r.Location = LocationCategory
	}
}

func (r MoveTaskRequest) Validate() error {
	switch r.Location {
	case LocationCategory:
		if r.CategoryID == "" {
			return fmt.Errorf("%w: categoryId required for category move", ErrInvalidRequest)
		}
	case LocationBackburner, LocationArchive:
	default:
		return ErrInvalidLocation
	}
	return nil
}

type FocusRequest struct {
	TaskID string `json:"taskId"`
}

type CategoryPatch struct {
	Name  *string  `json:"name,omitempty"`
	Order []string `json:"order,omitempty"`
}

type MoveCategoryRequest struct {
	Location string `json:"location"`
	Position *int   `json:"position,omitempty"`
}

func (r *MoveCategoryRequest) Normalize() {
	if r.Location == "" {
		r.Location = LocationCategoryBoard
	}
}

func (r MoveCategoryRequest) Validate() error {
	switch r.Location {
	case LocationCategoryBoard, LocationBackburner, LocationArchive:
		return nil
	default:
		return ErrInvalidLocation
	}
}
