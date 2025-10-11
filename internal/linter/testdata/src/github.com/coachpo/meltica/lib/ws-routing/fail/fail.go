package fail

import (
	_ "github.com/coachpo/meltica/market_data/framework/api" // want "framework package \"github.com/coachpo/meltica/lib/ws-routing/fail\" cannot import domain package \"github.com/coachpo/meltica/market_data/framework/api\""
)

func noop() {}
