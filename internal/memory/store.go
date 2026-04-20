// Package memory — store.go
// Manages all K-0 persistent data: Episodes, Knowledge, Entities, Reports.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/report"
)

// Store is the K-0 memory system root.
type Store struct {
	cfg *config.Config
}

// NewStore creates a Store and ensures all memory directories exist.
func NewStore(cfg *config.Config) *Store {
	s := &Store{cfg: cfg}
	s.ensureDirs()
	return s
}

// Episode records a single agent goal run.
type Episode struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Tasks     int       `json:"tasks"`
	Outcome   string    `json:"outcome"`
	Tags      []string  `json:"tags,omitempty"`
}

// SaveEpisode persists an episode to ~/.kiai/memory/episodes/YYYY-MM-DD/<id>.json
func (s *Store) SaveEpisode(ep Episode) error {
	ep.EndTime = time.Now()
	dir := filepath.Join(config.MemoryDir(s.cfg, "episodes"), time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("memory: mkdir episodes: %w", err)
	}
	return writeJSON(filepath.Join(dir, ep.ID+".json"), ep)
}

// SaveReport writes a provisional report to ~/.kiai/memory/reports/
func (s *Store) SaveReport(goalID string, r *report.Provisional) error {
	if r == nil {
		return nil
	}
	dir := config.MemoryDir(s.cfg, "reports")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("memory: mkdir reports: %w", err)
	}
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("provisional-%s-%s.md", date, goalID)
	return os.WriteFile(filepath.Join(dir, filename), []byte(r.Markdown), 0600)
}

// KnowledgeEntry is a distilled lesson from past episodes.
type KnowledgeEntry struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	CVE       string    `json:"cve,omitempty"`
	Tool      string    `json:"tool,omitempty"`
}

// AppendKnowledge adds a knowledge entry to the knowledge store.
func (s *Store) AppendKnowledge(entry KnowledgeEntry) error {
	entry.CreatedAt = time.Now()
	path := filepath.Join(config.MemoryDir(s.cfg, "knowledge"), "learnings.json")
	return appendJSON(path, entry)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func appendJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

func (s *Store) ensureDirs() {
	for _, d := range []string{"episodes", "knowledge", "entities", "reports", "cache"} {
		_ = os.MkdirAll(config.MemoryDir(s.cfg, d), 0700)
	}
}

// ListEpisodes returns recent episodes, sorted newest first, up to limit.
func (s *Store) ListEpisodes(limit int) []Episode {
	dir := config.MemoryDir(s.cfg, "episodes")
	var episodes []Episode
	// Walk all date directories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Read dirs in reverse (newest first assuming YYYY-MM-DD names)
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		dayDir := filepath.Join(dir, entries[i].Name())
		files, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}
		// Reverse order within day dir too
		for j := len(files) - 1; j >= 0; j-- {
			if filepath.Ext(files[j].Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dayDir, files[j].Name()))
			if err != nil {
				continue
			}
			var ep Episode
			if json.Unmarshal(data, &ep) == nil {
				episodes = append(episodes, ep)
				if len(episodes) >= limit {
					return episodes
				}
			}
		}
	}
	return episodes
}

// ListKnowledge returns all knowledge entries, newest first, up to limit.
func (s *Store) ListKnowledge(limit int) []KnowledgeEntry {
	path := filepath.Join(config.MemoryDir(s.cfg, "knowledge"), "learnings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []KnowledgeEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry KnowledgeEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	// Reverse: newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// ListReports returns report filenames, newest first, up to limit.
func (s *Store) ListReports(limit int) []string {
	dir := config.MemoryDir(s.cfg, "reports")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Filter .md files
	var reports []string
	for i := len(entries) - 1; i >= 0; i-- {
		if filepath.Ext(entries[i].Name()) == ".md" {
			reports = append(reports, entries[i].Name())
			if len(reports) >= limit {
				break
			}
		}
	}
	return reports
}
