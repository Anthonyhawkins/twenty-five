package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveCategoryToBackburnerClearsFocus(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "board.json")
	initial := `{
		"categories": [
			{
				"id": "cat1",
				"name": "Alpha",
				"tasks": [
					{"id":"task1","name":"One","description":"","notes":"","state":"todo","size":1,"focused":true}
				]
			}
		],
		"backburner": [],
		"archives": [],
		"categoryBackburner": [],
		"categoryArchives": []
	}`
	if err := os.WriteFile(dataPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	cat, board, err := store.MoveCategory("cat1", MoveCategoryRequest{Location: LocationBackburner})
	if err != nil {
		t.Fatalf("move category: %v", err)
	}
	if cat.ID != "cat1" {
		t.Fatalf("expected category id cat1, got %q", cat.ID)
	}
	if len(board.Categories) != 0 {
		t.Fatalf("expected active categories to be empty, got %d", len(board.Categories))
	}
	if len(board.CategoryBackburner) != 1 {
		t.Fatalf("expected one category in backburner, got %d", len(board.CategoryBackburner))
	}
	if len(board.CategoryBackburner[0].Tasks) != 1 {
		t.Fatalf("expected tasks to follow category to backburner")
	}
	if board.CategoryBackburner[0].Tasks[0].Focused {
		t.Fatalf("expected focused flag cleared on backburnered category tasks")
	}
}

func TestMoveCategoryBackToBoard(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "board.json")
	initial := `{
		"categories": [],
		"backburner": [],
		"archives": [],
		"categoryBackburner": [
			{
				"id": "cat1",
				"name": "Alpha",
				"tasks": [
					{"id":"task1","name":"One","description":"","notes":"","state":"todo","size":2,"focused":false}
				]
			}
		],
		"categoryArchives": []
	}`
	if err := os.WriteFile(dataPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	cat, board, err := store.MoveCategory("cat1", MoveCategoryRequest{Location: LocationCategoryBoard})
	if err != nil {
		t.Fatalf("restore category: %v", err)
	}
	if cat.ID != "cat1" {
		t.Fatalf("expected category id cat1, got %q", cat.ID)
	}
	if len(board.CategoryBackburner) != 0 {
		t.Fatalf("expected backburner to be empty, got %d", len(board.CategoryBackburner))
	}
	if len(board.Categories) != 1 {
		t.Fatalf("expected one active category, got %d", len(board.Categories))
	}
	if board.Categories[0].ID != "cat1" {
		t.Fatalf("unexpected active category id %q", board.Categories[0].ID)
	}
	if board.Categories[0].Tasks[0].Focused {
		t.Fatalf("expected tasks to remain unfocused when restored")
	}
}

func TestMoveCategoryRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "board.json")
	initial := `{
		"categories": [
			{"id":"cat1","name":"Alpha","tasks":[]},
			{"id":"cat2","name":"Beta","tasks":[]},
			{"id":"cat3","name":"Gamma","tasks":[]},
			{"id":"cat4","name":"Delta","tasks":[]},
			{"id":"cat5","name":"Epsilon","tasks":[]}
		],
		"backburner": [],
		"archives": [],
		"categoryBackburner": [
			{"id":"cat6","name":"Zeta","tasks":[]}
		],
		"categoryArchives": []
	}`
	if err := os.WriteFile(dataPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, _, err := store.MoveCategory("cat6", MoveCategoryRequest{Location: LocationCategoryBoard}); !errors.Is(err, ErrCategoryLimit) {
		t.Fatalf("expected ErrCategoryLimit, got %v", err)
	}
}

func TestCreateCategoryRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "board.json")
	initial := `{
		"categories": [
			{"id":"cat1","name":"Alpha","tasks":[]},
			{"id":"cat2","name":"Beta","tasks":[]},
			{"id":"cat3","name":"Gamma","tasks":[]},
			{"id":"cat4","name":"Delta","tasks":[]},
			{"id":"cat5","name":"Epsilon","tasks":[]}
		],
		"backburner": [],
		"archives": [],
		"categoryBackburner": [],
		"categoryArchives": []
	}`
	if err := os.WriteFile(dataPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, _, err := store.CreateCategory("Zeta"); !errors.Is(err, ErrCategoryLimit) {
		t.Fatalf("expected ErrCategoryLimit, got %v", err)
	}
}

func TestMoveCategoryCapacityCheck(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "board.json")
	initial := `{
		"categories": [
			{"id":"cat1","name":"Alpha","tasks":[]},
			{"id":"cat2","name":"Beta","tasks":[]},
			{"id":"cat3","name":"Gamma","tasks":[]},
			{"id":"cat4","name":"Delta","tasks":[]}
		],
		"backburner": [],
		"archives": [],
		"categoryBackburner": [
			{
				"id":"cat5",
				"name":"Overloaded",
				"tasks":[
					{"id":"task1","name":"One","description":"","notes":"","state":"todo","size":3},
					{"id":"task2","name":"Two","description":"","notes":"","state":"todo","size":3}
				]
			}
		],
		"categoryArchives": []
	}`
	if err := os.WriteFile(dataPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	store, err := NewStore(dataPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, _, err := store.MoveCategory("cat5", MoveCategoryRequest{Location: LocationCategoryBoard}); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("expected ErrCapacityExceeded, got %v", err)
	}
}
