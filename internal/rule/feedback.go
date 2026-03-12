package rule

type Feedback struct {
	Rule
	Message string
}

func NewFeedback(rule Rule, message string) Rule {
	if message == "" {
		return rule
	}

	return &Feedback{
		Rule:    rule,
		Message: message,
	}
}

func (feedback *Feedback) Init() Rule {
	return &Feedback{
		Rule:    feedback.Rule.Init(),
		Message: feedback.Message,
	}
}

func (feedback *Feedback) GetErrorMessage() string {
	if feedback.Message == "" {
		return feedback.Rule.GetErrorMessage()
	}

	return feedback.Message
}

func (feedback *Feedback) Copy() Rule {
	return &Feedback{
		Rule:    feedback.Rule.Copy(),
		Message: feedback.Message,
	}
}
