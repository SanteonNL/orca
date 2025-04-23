package careplanservice

type Policy interface {
	HasAccess() (bool, error)
}

type EveryoneHasAccessPolicy struct {
}

type OnlyCreatorHasAccess struct {
}
