package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu    sync.RWMutex
	state BoardState
	path  string
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.loadOrSeed(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadOrSeed() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.state = seedBoard()
			return s.saveLocked()
		}
		return fmt.Errorf("open data file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if len(data) == 0 {
		s.state = seedBoard()
		return s.saveLocked()
	}

	var loaded BoardState
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}

	normalizeBoardState(&loaded)
	s.state = loaded
	return nil
}

func (s *Store) GetState() BoardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Clone()
}

func normalizeBoardState(state *BoardState) {
	if state.Categories == nil {
		state.Categories = []Category{}
	}
	if state.Backburner == nil {
		state.Backburner = []Task{}
	}
	if state.Archives == nil {
		state.Archives = []Task{}
	}
	if state.CategoryBackburner == nil {
		state.CategoryBackburner = []Category{}
	}
	if state.CategoryArchives == nil {
		state.CategoryArchives = []Category{}
	}
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal board: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.path), "board-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *Store) withWrite(lockFn func(state *BoardState) error) (BoardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := lockFn(&s.state); err != nil {
		return BoardState{}, err
	}
	if err := s.saveLocked(); err != nil {
		return BoardState{}, err
	}
	return s.state.Clone(), nil
}

// CreateTask inserts a task into the requested location.
func (s *Store) CreateTask(req CreateTaskRequest) (Task, BoardState, error) {
	req.Normalize()
	if err := req.Validate(); err != nil {
		return Task{}, BoardState{}, err
	}

	var created Task
	updatedState, err := s.withWrite(func(state *BoardState) error {
		var err error
		created, err = state.insertTask(req)
		return err
	})
	if err != nil {
		return Task{}, BoardState{}, err
	}
	return created, updatedState, nil
}

func (s *Store) UpdateTask(id string, patch TaskPatch) (Task, BoardState, error) {
	var updated Task
	updatedState, err := s.withWrite(func(state *BoardState) error {
		taskPtr, loc, err := findTask(state, id)
		if err != nil {
			return err
		}
		if err := patch.Apply(taskPtr); err != nil {
			return err
		}
		if loc.Kind == LocationCategory {
			if taskPtr.Urgent {
				normalizeUrgent(state, loc.CategoryIndex, taskPtr.ID)
			} else {
				normalizeUrgent(state, loc.CategoryIndex, "")
			}
		}
		if loc.Kind == LocationCategory {
			if err := ensureCapacity(state.Categories[loc.CategoryIndex]); err != nil {
				return err
			}
		}
		updated = taskPtr.Clone()
		return nil
	})
	if err != nil {
		return Task{}, BoardState{}, err
	}
	return updated, updatedState, nil
}

func (s *Store) MoveTask(id string, dest MoveTaskRequest) (Task, BoardState, error) {
	var moved Task
	updatedState, err := s.withWrite(func(state *BoardState) error {
		task, loc, err := removeTask(state, id)
		if err != nil {
			return err
		}

		destCopy := dest
		if (destCopy.Location == LocationBackburner || destCopy.Location == LocationArchive) && destCopy.SourceID == "" {
			if loc.Kind == LocationCategory {
				cat := state.Categories[loc.CategoryIndex]
				destCopy.SourceID = cat.ID
				destCopy.Source = cat.Name
			}
		}

		if err := state.placeTask(task, destCopy); err != nil {
			// reinsert original task to preserve state
			restoreTask(state, task, loc)
			return err
		}
		moved = task.Clone()
		return nil
	})
	if err != nil {
		return Task{}, BoardState{}, err
	}
	return moved, updatedState, nil
}

func (s *Store) DeleteTask(id string) (BoardState, error) {
	updatedState, err := s.withWrite(func(state *BoardState) error {
		_, loc, err := findTask(state, id)
		if err != nil {
			return err
		}
		if loc.Kind != LocationArchive {
			return fmt.Errorf("task %s is not in archive", id)
		}
		_, _, err = removeTask(state, id)
		return err
	})
	return updatedState, err
}

func (s *Store) RenameCategory(id, name string) (Category, BoardState, error) {
	var cat Category
	updatedState, err := s.withWrite(func(state *BoardState) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%w: name cannot be empty", ErrInvalidRequest)
		}
		for _, existing := range state.Categories {
			if existing.Name == name && existing.ID != id {
				return ErrDuplicateCategory
			}
		}
		for _, existing := range state.CategoryBackburner {
			if existing.Name == name && existing.ID != id {
				return ErrDuplicateCategory
			}
		}
		for _, existing := range state.CategoryArchives {
			if existing.Name == name && existing.ID != id {
				return ErrDuplicateCategory
			}
		}
		for i := range state.Categories {
			if state.Categories[i].ID == id {
				state.Categories[i].Name = name
				cat = state.Categories[i].Clone()
				return nil
			}
		}
		return ErrCategoryNotFound
	})
	if err != nil {
		return Category{}, BoardState{}, err
	}
	return cat, updatedState, nil
}

func (s *Store) CreateCategory(name string) (Category, BoardState, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, BoardState{}, fmt.Errorf("%w: name required", ErrInvalidRequest)
	}
	var cat Category
	updatedState, err := s.withWrite(func(state *BoardState) error {
		for _, existing := range state.Categories {
			if existing.Name == name {
				return ErrDuplicateCategory
			}
		}
		for _, existing := range state.CategoryBackburner {
			if existing.Name == name {
				return ErrDuplicateCategory
			}
		}
		for _, existing := range state.CategoryArchives {
			if existing.Name == name {
				return ErrDuplicateCategory
			}
		}
		if len(state.Categories) >= CategoryLimit {
			return ErrCategoryLimit
		}
		cat = Category{
			ID:    NewID(),
			Name:  name,
			Tasks: []Task{},
		}
		state.Categories = append(state.Categories, cat)
		return nil
	})
	if err != nil {
		return Category{}, BoardState{}, err
	}
	return cat, updatedState, nil
}

func (s *Store) MoveCategory(id string, dest MoveCategoryRequest) (Category, BoardState, error) {
	var moved Category
	updatedState, err := s.withWrite(func(state *BoardState) error {
		dest.Normalize()
		if err := dest.Validate(); err != nil {
			return err
		}
		cat, loc, err := removeCategory(state, id)
		if err != nil {
			return err
		}
		if err := state.placeCategory(cat, dest); err != nil {
			restoreCategory(state, cat, loc)
			return err
		}
		moved = cat.Clone()
		return nil
	})
	if err != nil {
		return Category{}, BoardState{}, err
	}
	return moved, updatedState, nil
}

func (s *Store) ReorderCategoryTasks(id string, order []string) (Category, BoardState, error) {
	var cat Category
	updatedState, err := s.withWrite(func(state *BoardState) error {
		for i := range state.Categories {
			if state.Categories[i].ID == id {
				if err := reorderTasks(&state.Categories[i], order); err != nil {
					return err
				}
				cat = state.Categories[i].Clone()
				return nil
			}
		}
		return ErrCategoryNotFound
	})
	if err != nil {
		return Category{}, BoardState{}, err
	}
	return cat, updatedState, nil
}

func (s *Store) SetFocused(taskID string) (Task, BoardState, error) {
	var focused Task
	updatedState, err := s.withWrite(func(state *BoardState) error {
		if taskID == "" {
			clearFocus(state)
			return nil
		}
		taskPtr, _, err := findTask(state, taskID)
		if err != nil {
			return err
		}
		clearFocus(state)
		taskPtr.Focused = true
		focused = taskPtr.Clone()
		return nil
	})
	if err != nil {
		return Task{}, BoardState{}, err
	}
	return focused, updatedState, nil
}

func reorderTasks(cat *Category, order []string) error {
	if len(order) != len(cat.Tasks) {
		return fmt.Errorf("%w: task order length mismatch", ErrInvalidRequest)
	}
	index := map[string]int{}
	for i, id := range order {
		index[id] = i
	}
	reordered := make([]Task, len(cat.Tasks))
	for _, task := range cat.Tasks {
		pos, ok := index[task.ID]
		if !ok {
			return fmt.Errorf("%w: missing task id %s", ErrInvalidRequest, task.ID)
		}
		reordered[pos] = task
	}
	copy(cat.Tasks, reordered)
	return nil
}

func ensureCapacity(cat Category) error {
	total := 0
	for _, t := range cat.Tasks {
		total += t.Size
		if total > ColumnCapacity {
			return ErrCapacityExceeded
		}
	}
	return nil
}

func normalizeUrgent(state *BoardState, catIdx int, urgentTaskID string) {
	for i := range state.Categories[catIdx].Tasks {
		state.Categories[catIdx].Tasks[i].Urgent = state.Categories[catIdx].Tasks[i].ID == urgentTaskID
	}
}

func normalizeFocus(state *BoardState, focusedID string) {
	for i := range state.Categories {
		for j := range state.Categories[i].Tasks {
			state.Categories[i].Tasks[j].Focused = state.Categories[i].Tasks[j].ID == focusedID && focusedID != ""
		}
	}
	for i := range state.Backburner {
		state.Backburner[i].Focused = false
	}
	for i := range state.Archives {
		state.Archives[i].Focused = false
	}
	for i := range state.CategoryBackburner {
		for j := range state.CategoryBackburner[i].Tasks {
			state.CategoryBackburner[i].Tasks[j].Focused = false
		}
	}
	for i := range state.CategoryArchives {
		for j := range state.CategoryArchives[i].Tasks {
			state.CategoryArchives[i].Tasks[j].Focused = false
		}
	}
}

func clearFocus(state *BoardState) {
	for i := range state.Categories {
		for j := range state.Categories[i].Tasks {
			state.Categories[i].Tasks[j].Focused = false
		}
	}
	for i := range state.Backburner {
		state.Backburner[i].Focused = false
	}
	for i := range state.Archives {
		state.Archives[i].Focused = false
	}
	for i := range state.CategoryBackburner {
		for j := range state.CategoryBackburner[i].Tasks {
			state.CategoryBackburner[i].Tasks[j].Focused = false
		}
	}
	for i := range state.CategoryArchives {
		for j := range state.CategoryArchives[i].Tasks {
			state.CategoryArchives[i].Tasks[j].Focused = false
		}
	}
}

func clearCategoryFocus(cat *Category) {
	for i := range cat.Tasks {
		cat.Tasks[i].Focused = false
	}
}

func restoreTask(state *BoardState, task Task, loc taskLocation) {
	switch loc.Kind {
	case LocationCategory:
		cat := &state.Categories[loc.CategoryIndex]
		if loc.TaskIndex >= len(cat.Tasks) {
			cat.Tasks = append(cat.Tasks, task)
		} else {
			cat.Tasks = append(cat.Tasks[:loc.TaskIndex+1], cat.Tasks[loc.TaskIndex:]...)
			cat.Tasks[loc.TaskIndex] = task
		}
	case LocationBackburner:
		state.Backburner = append(state.Backburner, Task{})
		copy(state.Backburner[loc.TaskIndex+1:], state.Backburner[loc.TaskIndex:])
		state.Backburner[loc.TaskIndex] = task
	case LocationArchive:
		state.Archives = append(state.Archives, Task{})
		copy(state.Archives[loc.TaskIndex+1:], state.Archives[loc.TaskIndex:])
		state.Archives[loc.TaskIndex] = task
	}
}

type categoryLocation struct {
	Kind  string
	Index int
}

func removeCategory(state *BoardState, id string) (Category, categoryLocation, error) {
	for i := range state.Categories {
		if state.Categories[i].ID == id {
			cat := state.Categories[i].Clone()
			state.Categories = append(state.Categories[:i], state.Categories[i+1:]...)
			return cat, categoryLocation{Kind: LocationCategoryBoard, Index: i}, nil
		}
	}
	for i := range state.CategoryBackburner {
		if state.CategoryBackburner[i].ID == id {
			cat := state.CategoryBackburner[i].Clone()
			state.CategoryBackburner = append(state.CategoryBackburner[:i], state.CategoryBackburner[i+1:]...)
			return cat, categoryLocation{Kind: LocationBackburner, Index: i}, nil
		}
	}
	for i := range state.CategoryArchives {
		if state.CategoryArchives[i].ID == id {
			cat := state.CategoryArchives[i].Clone()
			state.CategoryArchives = append(state.CategoryArchives[:i], state.CategoryArchives[i+1:]...)
			return cat, categoryLocation{Kind: LocationArchive, Index: i}, nil
		}
	}
	return Category{}, categoryLocation{}, ErrCategoryNotFound
}

func restoreCategory(state *BoardState, cat Category, loc categoryLocation) {
	switch loc.Kind {
	case LocationCategoryBoard:
		if loc.Index >= len(state.Categories) {
			state.Categories = append(state.Categories, cat)
			return
		}
		state.Categories = append(state.Categories, Category{})
		copy(state.Categories[loc.Index+1:], state.Categories[loc.Index:])
		state.Categories[loc.Index] = cat
	case LocationBackburner:
		state.CategoryBackburner = append(state.CategoryBackburner, Category{})
		copy(state.CategoryBackburner[loc.Index+1:], state.CategoryBackburner[loc.Index:])
		state.CategoryBackburner[loc.Index] = cat
	case LocationArchive:
		state.CategoryArchives = append(state.CategoryArchives, Category{})
		copy(state.CategoryArchives[loc.Index+1:], state.CategoryArchives[loc.Index:])
		state.CategoryArchives[loc.Index] = cat
	}
}

type taskLocation struct {
	Kind          string
	CategoryIndex int
	TaskIndex     int
}

func findTask(state *BoardState, id string) (*Task, taskLocation, error) {
	for ci := range state.Categories {
		for ti := range state.Categories[ci].Tasks {
			if state.Categories[ci].Tasks[ti].ID == id {
				return &state.Categories[ci].Tasks[ti], taskLocation{Kind: LocationCategory, CategoryIndex: ci, TaskIndex: ti}, nil
			}
		}
	}
	for i := range state.Backburner {
		if state.Backburner[i].ID == id {
			return &state.Backburner[i], taskLocation{Kind: LocationBackburner, TaskIndex: i}, nil
		}
	}
	for i := range state.Archives {
		if state.Archives[i].ID == id {
			return &state.Archives[i], taskLocation{Kind: LocationArchive, TaskIndex: i}, nil
		}
	}
	return nil, taskLocation{}, ErrTaskNotFound
}

func removeTask(state *BoardState, id string) (Task, taskLocation, error) {
	if taskPtr, loc, err := findTask(state, id); err == nil {
		task := taskPtr.Clone()
		switch loc.Kind {
		case LocationCategory:
			cat := &state.Categories[loc.CategoryIndex]
			cat.Tasks = append(cat.Tasks[:loc.TaskIndex], cat.Tasks[loc.TaskIndex+1:]...)
		case LocationBackburner:
			state.Backburner = append(state.Backburner[:loc.TaskIndex], state.Backburner[loc.TaskIndex+1:]...)
		case LocationArchive:
			state.Archives = append(state.Archives[:loc.TaskIndex], state.Archives[loc.TaskIndex+1:]...)
		}
		return task, loc, nil
	}
	return Task{}, taskLocation{}, ErrTaskNotFound
}

func (state *BoardState) insertTask(req CreateTaskRequest) (Task, error) {
	task := req.Task
	if task.ID == "" {
		task.ID = NewID()
	}
	if task.Size == 0 {
		task.Size = 1
	}
	var err error
	task.Size, err = NormalizeSize(task.Size)
	if err != nil {
		return Task{}, err
	}
	if err := ValidateTaskState(task.State); err != nil {
		return Task{}, err
	}

	switch req.Location {
	case LocationCategory:
		idx := findCategoryIndex(state.Categories, req.CategoryID)
		if idx == -1 {
			return Task{}, ErrCategoryNotFound
		}
		insertIndex := len(state.Categories[idx].Tasks)
		if req.Position != nil && *req.Position >= 0 && *req.Position <= len(state.Categories[idx].Tasks) {
			insertIndex = *req.Position
		}
		cat := &state.Categories[idx]
		cat.Tasks = append(cat.Tasks, Task{})
		copy(cat.Tasks[insertIndex+1:], cat.Tasks[insertIndex:])
		cat.Tasks[insertIndex] = task
		if err := ensureCapacity(*cat); err != nil {
			cat.Tasks = append(cat.Tasks[:insertIndex], cat.Tasks[insertIndex+1:]...)
			return Task{}, err
		}
		if task.Urgent {
			normalizeUrgent(state, idx, task.ID)
		}
		if task.Focused {
			normalizeFocus(state, task.ID)
		}
	case LocationBackburner:
		task.Urgent = false
		task.Focused = false
		state.Backburner = append(state.Backburner, task)
	case LocationArchive:
		task.Urgent = false
		task.Focused = false
		state.Archives = append(state.Archives, task)
	default:
		return Task{}, ErrInvalidLocation
	}
	return task.Clone(), nil
}

func (state *BoardState) placeTask(task Task, dest MoveTaskRequest) error {
	dest.Normalize()
	if err := dest.Validate(); err != nil {
		return err
	}

	task.Focused = false

	switch dest.Location {
	case LocationCategory:
		idx := findCategoryIndex(state.Categories, dest.CategoryID)
		if idx == -1 {
			return ErrCategoryNotFound
		}
		cat := &state.Categories[idx]
		insertIndex := len(cat.Tasks)
		if dest.Position != nil && *dest.Position >= 0 && *dest.Position <= len(cat.Tasks) {
			insertIndex = *dest.Position
		}
		task.SourceID = ""
		task.Source = ""
		if task.Urgent {
			normalizeUrgent(state, idx, task.ID)
		} else {
			normalizeUrgent(state, idx, "")
		}
		cat.Tasks = append(cat.Tasks, Task{})
		copy(cat.Tasks[insertIndex+1:], cat.Tasks[insertIndex:])
		cat.Tasks[insertIndex] = task
		if err := ensureCapacity(*cat); err != nil {
			cat.Tasks = append(cat.Tasks[:insertIndex], cat.Tasks[insertIndex+1:]...)
			return err
		}
	case LocationBackburner:
		task.Urgent = false
		task.Focused = false
		task.SourceID = dest.SourceID
		task.Source = dest.Source
		state.Backburner = append(state.Backburner, task)
	case LocationArchive:
		task.Urgent = false
		task.Focused = false
		task.SourceID = dest.SourceID
		task.Source = dest.Source
		state.Archives = append(state.Archives, task)
	default:
		return ErrInvalidLocation
	}
	return nil
}

func (state *BoardState) placeCategory(cat Category, dest MoveCategoryRequest) error {
	switch dest.Location {
	case LocationCategoryBoard:
		if len(state.Categories) >= CategoryLimit {
			return ErrCategoryLimit
		}
		if err := ensureCapacity(cat); err != nil {
			return err
		}
		insertIndex := len(state.Categories)
		if dest.Position != nil && *dest.Position >= 0 && *dest.Position <= len(state.Categories) {
			insertIndex = *dest.Position
		}
		state.Categories = append(state.Categories, Category{})
		copy(state.Categories[insertIndex+1:], state.Categories[insertIndex:])
		state.Categories[insertIndex] = cat
	case LocationBackburner:
		clearCategoryFocus(&cat)
		state.CategoryBackburner = append(state.CategoryBackburner, cat)
	case LocationArchive:
		clearCategoryFocus(&cat)
		state.CategoryArchives = append(state.CategoryArchives, cat)
	default:
		return ErrInvalidLocation
	}
	return nil
}

func findCategoryIndex(categories []Category, id string) int {
	for i := range categories {
		if categories[i].ID == id {
			return i
		}
	}
	return -1
}

func seedBoard() BoardState {
	newTask := func(name, desc, state string, size int) Task {
		return Task{
			ID:          NewID(),
			Name:        name,
			Description: desc,
			State:       state,
			Size:        size,
		}
	}
	newCategory := func(name string, tasks []Task) Category {
		return Category{
			ID:    NewID(),
			Name:  name,
			Tasks: tasks,
		}
	}

	return BoardState{
		Categories: []Category{
			newCategory("Backlog", []Task{
				newTask("Research idea", "Collect references and create a quick outline.", "todo", 2),
				newTask("Spike prototype", "Throwaway exploration to de-risk approach.", "doing", 2),
				newTask("Triage inbox", "Sort and label incoming items.", "todo", 1),
			}),
			newCategory("Planning", []Task{
				newTask("Roadmap v1", "Define milestones and success criteria.", "doing", 3),
				newTask("Team sync", "Align on goals, scope, and timelines.", "todo", 2),
			}),
			newCategory("Build", []Task{
				newTask("Implement core", "Core feature work across modules.", "doing", 2),
				newTask("Write tests", "Unit and integration coverage.", "todo", 1),
				newTask("Docs pass", "README + usage examples.", "todo", 2),
			}),
			newCategory("Launch", []Task{
				newTask("Release prep", "Changelog, versioning, and rollout plan.", "todo", 5),
			}),
			newCategory("Personal", []Task{
				newTask("Workout", "Split day routine; 45 minutes.", "doing", 2),
				newTask("Groceries", "Restock staples and veggies.", "todo", 1),
				newTask("Call mom", "Weekly check-in.", "done", 1),
				newTask("Read 30 mins", "Continue current book.", "todo", 1),
			}),
		},
		Backburner:         []Task{},
		Archives:           []Task{},
		CategoryBackburner: []Category{},
		CategoryArchives:   []Category{},
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
