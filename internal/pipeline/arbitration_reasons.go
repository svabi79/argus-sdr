package pipeline

const (
	ReasonAdmissionNoCandidates = "admission:none:candidates"
	ReasonAdmissionDisabled     = "admission:disabled"
)

const (
	DecisionReasonRecordClass = "decision:record:class"
	DecisionReasonRecordHint  = "decision:record:hint"
	DecisionReasonDecodeClass = "decision:decode:class"
	DecisionReasonDecodeHint  = "decision:decode:hint"
	DecisionReasonHintOnly    = "decision:hint"
	DecisionReasonQueueRecord = "queue:record:budget"
	DecisionReasonQueueDecode = "queue:decode:budget"
	DecisionReasonUnspecified = "decision:unspecified"
)

const (
	HoldReasonProfileArchive     = "profile:archive"
	HoldReasonProfileDigital     = "profile:digital"
	HoldReasonProfileAggressive  = "profile:aggressive"
	HoldReasonStrategyArchive    = "strategy:archive"
	HoldReasonStrategyDigital    = "strategy:digital"
	HoldReasonStrategyMultiRes   = "strategy:multi-resolution"
	HoldReasonIntentArchive      = "intent:archive"
	HoldReasonIntentDecode       = "intent:decode"
	HoldReasonIntentSurveillance = "intent:surveillance"
)

const (
	ReasonTagHoldActive          = "hold:active"
	ReasonTagHoldExpired         = "hold:expired"
	ReasonTagHoldProtected       = "hold:protected"
	ReasonTagHoldDisplaced       = "hold:displaced"
	ReasonTagDisplaceOpportunist = "displace:opportunistic"
	ReasonTagDisplaceTier        = "displace:tier"
)
