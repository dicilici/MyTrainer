package selector

type Selector interface {
	selectData(sampling bool) error
}

type Criteria struct {
	Field    string `json:"Field"`
	Operator string `json:"Operator"`
	Value    string `json:"Value"`
}

func NewCriteria(field string, operator string, value string) *Criteria {
	return &Criteria{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
}

type DefaultSelector struct {
	Conditions []Criteria
}

func NewDefaultSelector(conditions ...Criteria) *DefaultSelector {
	return &DefaultSelector{
		Conditions: conditions,
	}
}

func (s *DefaultSelector) selectData(sampling bool) error {
	return nil
}
