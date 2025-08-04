package hivesim

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang-jwt/jwt/v4"
)

// This is the static secret configured for all execution-layer clients.
var ENGINEAPI_JWT_SECRET = [32]byte{0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x65}

func jwtAuth(secret [32]byte) rpc.HTTPAuth {
	return func(h http.Header) error {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iat": &jwt.NumericDate{Time: time.Now()},
		})
		s, err := token.SignedString(secret[:])
		if err != nil {
			return fmt.Errorf("failed to create JWT token: %w", err)
		}
		h.Set("Authorization", "Bearer "+s)
		return nil
	}
}
