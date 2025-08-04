package globals

var (
	// Engine API errors
	SERVER_ERROR         = pInt(-32000)
	INVALID_REQUEST      = pInt(-32600)
	METHOD_NOT_FOUND     = pInt(-32601)
	INVALID_PARAMS_ERROR = pInt(-32602)
	INTERNAL_ERROR       = pInt(-32603)
	INVALID_JSON         = pInt(-32700)

	UNKNOWN_PAYLOAD            = pInt(-38001)
	INVALID_FORKCHOICE_STATE   = pInt(-38002)
	INVALID_PAYLOAD_ATTRIBUTES = pInt(-38003)
	TOO_LARGE_REQUEST          = pInt(-38004)
	UNSUPPORTED_FORK_ERROR     = pInt(-38005)
)

func pInt(v int) *int {
	return &v
}
