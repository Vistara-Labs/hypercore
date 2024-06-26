package containerd

import (
	"errors"
	"fmt"
	"vistara-node/pkg/ports"

	"github.com/containerd/containerd/events"
	"github.com/containerd/typeurl/v2"
)

// ErrReadingContent is used when there is an error reading from the content store.
var ErrReadingContent = errors.New("failed reading from content store")

func convertCtrEventEnvelope(evt *events.Envelope) (*ports.EventEnvelope, error) {
	if evt == nil {
		return nil, nil
	}

	converted := &ports.EventEnvelope{
		Timestamp: evt.Timestamp,
		Namespace: evt.Namespace,
		Topic:     evt.Topic,
	}

	v, err := typeurl.UnmarshalAny(evt.Event)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling event: %w", err)
	}

	converted.Event = v

	return converted, nil
}
