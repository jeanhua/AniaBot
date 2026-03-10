package llmtool

type BaseTool[T any] struct {
	name        string
	description string
	params      T
}

func (t *BaseTool[T]) Name() string {
	return t.name
}

func (t *BaseTool[T]) Description() string {
	return t.description
}

func (t *BaseTool[T]) Params() any {
	return &t.params
}

func MakeBaseTool[T any](name, description string, params T) BaseTool[T] {
	return BaseTool[T]{
		name:        name,
		description: description,
		params:      params,
	}
}
