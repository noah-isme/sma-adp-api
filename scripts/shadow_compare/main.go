package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type target struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Critical bool   `json:"critical"`
}

type config struct {
	Targets []target `json:"targets"`
}

type comparison struct {
	Target         target
	LegacyStatus   int
	GoStatus       int
	StatusMatch    bool
	BodyMatch      bool
	Error          error
	DurationGo     time.Duration
	DurationLegacy time.Duration
}

func main() {
	var (
		goBase      string
		legacyBase  string
		targetsPath string
		timeout     time.Duration
	)

	flag.StringVar(&goBase, "go-base", "http://localhost:8080", "Go API base URL")
	flag.StringVar(&legacyBase, "legacy-base", "http://localhost:3000", "Legacy API base URL")
	flag.StringVar(&targetsPath, "targets", filepath.Join("scripts", "shadow_compare", "targets.json"), "Path to JSON targets file")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "HTTP client timeout")
	flag.Parse()

	targets, err := loadTargets(targetsPath)
	if err != nil {
		log.Fatalf("failed to load targets: %v", err)
	}

	client := &http.Client{Timeout: timeout}
	var (
		comparisons  []comparison
		breaking     int
		optionalDiff int
	)

	for _, t := range targets {
		comp := compareTarget(client, goBase, legacyBase, t)
		if comp.Error != nil {
			if t.Critical {
				breaking++
			}
		} else {
			if !comp.StatusMatch || !comp.BodyMatch {
				if t.Critical {
					breaking++
				} else {
					optionalDiff++
				}
			}
		}
		comparisons = append(comparisons, comp)
	}

	printReport(comparisons)

	fmt.Printf("Breaking diffs: %d, Optional diffs: %d\n", breaking, optionalDiff)
	if breaking > 0 {
		os.Exit(1)
	}
}

func loadTargets(path string) ([]target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no targets defined in %s", path)
	}
	return cfg.Targets, nil
}

func compareTarget(client *http.Client, goBase, legacyBase string, tgt target) comparison {
	comp := comparison{Target: tgt}
	goResp, goDur, goErr := performRequest(client, goBase, tgt)
	legacyResp, legacyDur, legacyErr := performRequest(client, legacyBase, tgt)
	comp.DurationGo = goDur
	comp.DurationLegacy = legacyDur

	if goErr != nil {
		comp.Error = fmt.Errorf("go request failed: %w", goErr)
		return comp
	}
	if legacyErr != nil {
		comp.Error = fmt.Errorf("legacy request failed: %w", legacyErr)
		return comp
	}

	comp.GoStatus = goResp.StatusCode
	comp.LegacyStatus = legacyResp.StatusCode
	comp.StatusMatch = comp.GoStatus == comp.LegacyStatus

	defer goResp.Body.Close()
	defer legacyResp.Body.Close()

	goBody, err := io.ReadAll(goResp.Body)
	if err != nil {
		comp.Error = fmt.Errorf("read go body: %w", err)
		return comp
	}
	legacyBody, err := io.ReadAll(legacyResp.Body)
	if err != nil {
		comp.Error = fmt.Errorf("read legacy body: %w", err)
		return comp
	}

	comp.BodyMatch = bodiesEqual(goBody, legacyBody)

	return comp
}

func performRequest(client *http.Client, base string, tgt target) (*http.Response, time.Duration, error) {
	if client == nil {
		return nil, 0, errors.New("nil client")
	}
	method := strings.ToUpper(strings.TrimSpace(tgt.Method))
	if method == "" {
		method = http.MethodGet
	}
	path := tgt.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := strings.TrimRight(base, "/") + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	return resp, time.Since(start), nil
}

func bodiesEqual(a, b []byte) bool {
	if bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b)) {
		return true
	}

	var aj, bj interface{}
	if err := json.Unmarshal(a, &aj); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bj); err != nil {
		return false
	}
	normalize(&aj)
	normalize(&bj)
	return reflect.DeepEqual(aj, bj)
}

func normalize(v *interface{}) {
	switch val := (*v).(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			normalize(&v2)
			val[k] = v2
		}
	case []interface{}:
		for i, v2 := range val {
			normalize(&v2)
			val[i] = v2
		}
	case float64:
		if val == float64(int64(val)) {
			*v = int64(val)
		}
	}
}

func printReport(results []comparison) {
	fmt.Println("Shadow Compare Report")
	fmt.Println("======================")
	for _, res := range results {
		status := "OK"
		if res.Error != nil {
			status = "ERROR"
		} else if !res.StatusMatch || !res.BodyMatch {
			status = "DIFF"
		}
		fmt.Printf("[%s] %s %s\n", status, res.Target.Method, res.Target.Path)
		fmt.Printf("  Go Status: %d (%s)\n", res.GoStatus, res.DurationGo)
		fmt.Printf("  Legacy Status: %d (%s)\n", res.LegacyStatus, res.DurationLegacy)
		if res.Error != nil {
			fmt.Printf("  Error: %v\n", res.Error)
		} else {
			fmt.Printf("  Status match: %t | Body match: %t | Critical: %t\n", res.StatusMatch, res.BodyMatch, res.Target.Critical)
		}
	}
}
