package httpserver

import (
	"net/http"
	"sort"
	"strings"
)

func withMutableRouteAuth(next http.Handler, token string) http.Handler {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutableControlPlaneRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if bearerToken(r) != trimmedToken {
			w.Header().Set("WWW-Authenticate", `Bearer realm="meltica-control-plane"`)
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func isMutableControlPlaneRequest(r *http.Request) bool {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	switch path {
	case strategyModulesPath:
		return r.Method == http.MethodPost
	case strategyRefreshPath:
		return r.Method == http.MethodPost
	case providersPath:
		return r.Method == http.MethodPost
	case instancesPath:
		return r.Method == http.MethodPost
	case riskLimitsPath:
		return r.Method == http.MethodPut
	case contextBackupPath:
		return r.Method == http.MethodPost
	}
	return isMutableStrategyModuleRequest(r.Method, path) ||
		isMutableProviderRequest(r.Method, path) ||
		isMutableInstanceRequest(r.Method, path)
}

func isMutableStrategyModuleRequest(method, path string) bool {
	if !strings.HasPrefix(path, strategyModulePrefix) {
		return false
	}
	segments := stripModuleRoutePrefix(splitPathSegments(strings.Trim(path, "/")))
	if len(segments) == 1 {
		return method == http.MethodPut || method == http.MethodDelete
	}
	if len(segments) == 3 && strings.EqualFold(segments[1], "tags") {
		return method == http.MethodPut || method == http.MethodDelete
	}
	return false
}

func isMutableProviderRequest(method, path string) bool {
	if !strings.HasPrefix(path, providerDetailPrefix) {
		return false
	}
	rest := strings.Trim(strings.TrimPrefix(path, providerDetailPrefix), "/")
	if rest == "" {
		return false
	}
	_, action, hasAction := strings.Cut(rest, "/")
	if !hasAction {
		return method == http.MethodPut || method == http.MethodDelete
	}
	return method == http.MethodPost && (action == "start" || action == "stop")
}

func isMutableInstanceRequest(method, path string) bool {
	if !strings.HasPrefix(path, instanceDetailPrefix) {
		return false
	}
	rest := strings.Trim(strings.TrimPrefix(path, instanceDetailPrefix), "/")
	if rest == "" {
		return false
	}
	_, action, hasAction := strings.Cut(rest, "/")
	if !hasAction {
		return method == http.MethodPut || method == http.MethodDelete
	}
	return method == http.MethodPost && (action == "start" || action == "stop")
}

func (s *httpServer) methodHandlers(handlers map[string]handlerFunc) http.Handler {
	allowed := allowedMethods(handlers)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := handlers[r.Method]; ok {
			handler(w, r)
			return
		}
		methodNotAllowed(w, allowed...)
	})
}

func allowedMethods(handlers map[string]handlerFunc) []string {
	if len(handlers) == 0 {
		return nil
	}
	allowed := make([]string, 0, len(handlers))
	for method := range handlers {
		allowed = append(allowed, method)
	}
	sort.Strings(allowed)
	return allowed
}
