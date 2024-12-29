package service

type orderService struct {
}

func NewOrderService() IOrderService {
	return &orderService{}
}

func (s *orderService) Get(id string) interface{} {
	return nil
}
