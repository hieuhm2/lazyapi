package storage

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store struct {
	dir string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".config", "lazyapi")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func NewID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (s *Store) LoadCollections() ([]Collection, error) {
	path := filepath.Join(s.dir, "collections.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultCollections(), nil
	}
	if err != nil {
		return nil, err
	}
	var cols []Collection
	if err := json.Unmarshal(data, &cols); err != nil {
		return nil, err
	}
	return cols, nil
}

func (s *Store) SaveCollections(cols []Collection) error {
	path := filepath.Join(s.dir, "collections.json")
	data, err := json.MarshalIndent(cols, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Store) LoadEnvFile() (EnvFile, error) {
	path := filepath.Join(s.dir, "envs.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultEnvFile(), nil
	}
	if err != nil {
		return EnvFile{}, err
	}
	var ef EnvFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return EnvFile{}, err
	}
	return ef, nil
}

func (s *Store) SaveEnvFile(ef EnvFile) error {
	path := filepath.Join(s.dir, "envs.json")
	data, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ResolveEnv replaces {{VAR}} placeholders with values from env.
func ResolveEnv(s string, env *Environment) string {
	if env == nil {
		return s
	}
	for k, v := range env.Variables {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

func DefaultCollections() []Collection {
	return []Collection{
		{
			ID:   NewID(),
			Name: "Examples",
			Requests: []Request{
				{
					ID:     NewID(),
					Name:   "List Users",
					Method: "GET",
					URL:    "https://jsonplaceholder.typicode.com/users",
				},
				{
					ID:     NewID(),
					Name:   "Create Post",
					Method: "POST",
					URL:    "https://jsonplaceholder.typicode.com/posts",
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: `{"title": "foo", "body": "bar", "userId": 1}`,
				},
				{
					ID:     NewID(),
					Name:   "Get Todo",
					Method: "GET",
					URL:    "https://jsonplaceholder.typicode.com/todos/1",
				},
			},
		},
		{
			ID:   NewID(),
			Name: "Local Dev",
			Requests: []Request{
				{
					ID:     NewID(),
					Name:   "Health Check",
					Method: "GET",
					URL:    "{{BASE_URL}}/health",
				},
			},
		},
	}
}

func DefaultEnvFile() EnvFile {
	return EnvFile{
		Active: "default",
		Envs: []Environment{
			{
				Name: "default",
				Variables: map[string]string{
					"BASE_URL": "http://localhost:8080",
					"TOKEN":    "your-token-here",
				},
			},
			{
				Name: "production",
				Variables: map[string]string{
					"BASE_URL": "https://api.example.com",
					"TOKEN":    "prod-token-here",
				},
			},
		},
	}
}
