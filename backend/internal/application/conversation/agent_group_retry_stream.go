package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// agentGroupRetryStreamID isolates each retry lease from the original run's
// replay retention. The persisted run ID stays unchanged for billing and history.
// Length framing keeps distinct (run, request) pairs unambiguous.
func agentGroupRetryStreamID(runID, retryRequestID string) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(len(runID)) + ":" + runID + retryRequestID))
	return hex.EncodeToString(sum[:])
}
