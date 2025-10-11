package api

type sessionFacade interface {
	Status() Status
	Subscriptions() []string
}

type sessionInspector struct {
	session sessionFacade
}

// WithSession adapts a routing session to the Inspector interface expected by the admin API handlers.
func WithSession(session sessionFacade) Inspector {
	return &sessionInspector{session: session}
}

func (s *sessionInspector) Status() Status {
	if s == nil || s.session == nil {
		return StatusStopped
	}
	return s.session.Status()
}

func (s *sessionInspector) Subscriptions() []Subscription {
	if s == nil || s.session == nil {
		return nil
	}
	topics := s.session.Subscriptions()
	items := make([]Subscription, len(topics))
	for i, topic := range topics {
		items[i] = Subscription{Symbol: topic}
	}
	return items
}
