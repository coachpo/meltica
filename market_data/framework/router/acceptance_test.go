package router

import "testing"

func TestUserStory1Acceptance(t *testing.T) {
	t.Run("AS1.1_MixedStreamRoutingAccuracy", TestMixedStreamRoutingAccuracy)
	t.Run("AS1.2_UnrecognizedMessageFallback", TestUnrecognizedMessageRoutesToDefault)
	t.Run("AS1.3_ConcurrentStreamsIsolation", TestConcurrentStreamsRoutingIsolation)
}
