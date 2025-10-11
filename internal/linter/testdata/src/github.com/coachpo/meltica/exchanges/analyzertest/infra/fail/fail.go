package fail

import (
	_ "github.com/coachpo/meltica/core/layers"
	_ "github.com/coachpo/meltica/exchanges/analyzertest/routing/router" // want "layer boundary violation: .*infra/fail.*cannot import .*routing/router"
)

func noop() {}
