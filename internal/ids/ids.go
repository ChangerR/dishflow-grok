package ids

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

func New() string {
	return strings.ToLower(ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader).String())
}
