package linter

import (
	"sort"
	"strings"
)

type layer string

const (
	layerUnknown    layer = "unknown"
	layerExternal   layer = "external"
	layerShared     layer = "shared"
	layerInterface  layer = "interface"
	layerConnection layer = "connection"
	layerRouting    layer = "routing"
	layerBusiness   layer = "business"
	layerFilter     layer = "filter"
)

const modulePath = "github.com/coachpo/meltica"

var allowedDependencies = map[layer]map[layer]struct{}{
	layerInterface: {
		layerInterface: {},
		layerShared:    {},
	},
	layerConnection: {
		layerConnection: {},
		layerInterface:  {},
		layerShared:     {},
	},
	layerRouting: {
		layerRouting:    {},
		layerConnection: {},
		layerInterface:  {},
		layerShared:     {},
	},
	layerBusiness: {
		layerBusiness:   {},
		layerRouting:    {},
		layerConnection: {},
		layerInterface:  {},
		layerShared:     {},
	},
	layerFilter: {
		layerFilter:     {},
		layerBusiness:   {},
		layerRouting:    {},
		layerConnection: {},
		layerInterface:  {},
		layerShared:     {},
	},
	layerShared: {
		layerShared:     {},
		layerInterface:  {},
		layerConnection: {},
		layerRouting:    {},
		layerBusiness:   {},
		layerFilter:     {},
	},
}

func resolveLayer(pkgPath string) layer {
	if pkgPath == "" {
		return layerUnknown
	}

	if !strings.HasPrefix(pkgPath, modulePath) {
		return layerExternal
	}

	rel := strings.TrimPrefix(pkgPath, modulePath)
	if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}

	switch {
	case strings.HasPrefix(rel, "core/layers"):
		return layerInterface
	case strings.Contains(rel, "/infra/"):
		return layerConnection
	case strings.Contains(rel, "/routing/"):
		return layerRouting
	case strings.Contains(rel, "/bridge/"):
		return layerBusiness
	case strings.Contains(rel, "/filter/"):
		return layerFilter
	case strings.HasPrefix(rel, "pipeline/"):
		return layerFilter
	default:
		return layerShared
	}
}

func isAllowedDependency(from, to layer) bool {
	if to == layerExternal || to == layerShared || to == layerUnknown {
		return true
	}

	if from == layerUnknown || from == layerExternal {
		return true
	}

	allowed, ok := allowedDependencies[from]
	if !ok {
		return true
	}

	_, ok = allowed[to]
	return ok
}

func allowedLayerNames(from layer) []string {
	allowed := allowedDependencies[from]
	if len(allowed) == 0 {
		return nil
	}

	var layers []string
	for l := range allowed {
		layers = append(layers, string(l))
	}
	sort.Strings(layers)
	return layers
}
