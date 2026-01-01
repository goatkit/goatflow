// Package main provides route diff utilities.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type route struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Handler    string   `json:"handler,omitempty"`
	RedirectTo string   `json:"redirectTo,omitempty"`
	Status     int      `json:"status,omitempty"`
	Websocket  bool     `json:"websocket,omitempty"`
	Middleware []string `json:"middleware,omitempty"`
}

type manifest struct {
	Routes []route `json:"routes"`
}

func key(r route) string { return r.Method + "|" + r.Path }

func load(path string) (map[string]route, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	out := map[string]route{}
	for _, r := range m.Routes {
		out[key(r)] = r
	}
	return out, nil
}

func main() {
	basePath := "generated/routes-manifest.baseline.json"
	curPath := "generated/routes-manifest.json"
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}
	if len(os.Args) > 2 {
		curPath = os.Args[2]
	}
	base, err := load(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading baseline: %v\n", err)
		os.Exit(1)
	}
	cur, err := load(curPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading current: %v\n", err)
		os.Exit(1)
	}
	added := []route{}
	removed := []route{}
	changed := []map[string]interface{}{}
	for k, r := range cur {
		if _, ok := base[k]; !ok {
			added = append(added, r)
		}
	}
	for k, r := range base {
		if _, ok := cur[k]; !ok {
			removed = append(removed, r)
		}
	}
	for k, b := range base {
		if c, ok := cur[k]; ok {
			diffFields := map[string][2]interface{}{}
			if b.Handler != c.Handler {
				diffFields["handler"] = [2]interface{}{b.Handler, c.Handler}
			}
			if b.RedirectTo != c.RedirectTo {
				diffFields["redirectTo"] = [2]interface{}{b.RedirectTo, c.RedirectTo}
			}
			if b.Status != c.Status {
				diffFields["status"] = [2]interface{}{b.Status, c.Status}
			}
			if b.Websocket != c.Websocket {
				diffFields["websocket"] = [2]interface{}{b.Websocket, c.Websocket}
			}
			// Middleware compare (order-insensitive)
			if !sameSet(b.Middleware, c.Middleware) {
				diffFields["middleware"] = [2]interface{}{b.Middleware, c.Middleware}
			}
			if len(diffFields) > 0 {
				changed = append(changed, map[string]interface{}{"method": b.Method, "path": b.Path, "changes": diffFields})
			}
		}
	}
	sort.Slice(added, func(i, j int) bool {
		if added[i].Method == added[j].Method {
			return added[i].Path < added[j].Path
		}
		return added[i].Method < added[j].Method
	})
	sort.Slice(removed, func(i, j int) bool {
		if removed[i].Method == removed[j].Method {
			return removed[i].Path < removed[j].Path
		}
		return removed[i].Method < removed[j].Method
	})
	sort.Slice(changed, func(i, j int) bool {
		if changed[i]["method"].(string) == changed[j]["method"].(string) {
			return changed[i]["path"].(string) < changed[j]["path"].(string)
		}
		return changed[i]["method"].(string) < changed[j]["method"].(string)
	})
	out := map[string]interface{}{"added": added, "removed": removed, "changed": changed}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]int{}
	bm := map[string]int{}
	for _, v := range a {
		am[v]++
	}
	for _, v := range b {
		bm[v]++
	}
	if len(am) != len(bm) {
		return false
	}
	for k, v := range am {
		if bm[k] != v {
			return false
		}
	}
	return true
}
