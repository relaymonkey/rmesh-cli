package climessages

// Meshtastic TEXT_MESSAGE_APP portnum (meshtastic/portnums.proto).
const TextPacketType = 1

// TextFilter is the structured filter for text message envelopes.
const TextFilter = "packet_type:eq:1"

// DefaultLimit matches GET /messages default (OpenAPI).
const DefaultLimit = 100
