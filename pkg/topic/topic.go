package topic

const (
	// BroadcastReply is the topic where replies to commands are published.
	BroadcastReply = "broadcast:reply"
	// BroadcastDebug is the topic where debug messages are published.
	BroadcastDebug = "broadcast:debug"
	// BroadcastDiag is the topic where diagnostic messages are published.
	BroadcastDiag = "broadcast:diag"

	// ReceiveCmdSerial is the topic where commands received from serial input are published.
	ReceiveCmdSerial = "rxcmd:serial"
)
