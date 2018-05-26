package main

type clientState interface {
	onBind()
	onBindSuccess()
	onTimeout()
	onNewMessageToPlayer()
}

// clientStateNotbinded
type clientStateNotbinded struct {
	client *Client
	item   *clientAuthTimeoutItem
}

func newClientStateNotbinded(c *Client) *clientStateNotbinded {
	s := &clientStateNotbinded{
		client: c,
	}
	s.item = &clientAuthTimeoutItem{c}
	// 等待auth消息接收超时
	serverInst.waitAuth(s.item)
	return s
}

func (s *clientStateNotbinded) onBind() {
	// 取消等待auth消息接收超时
	serverInst.stopWaitAuth(s.item)

	s.client.setState(newClientStateBinding(s.client))
}
func (s *clientStateNotbinded) onBindSuccess() {}
func (s *clientStateNotbinded) onTimeout() {
	s.client.close()
}
func (s *clientStateNotbinded) onNewMessageToPlayer() {}

// clientStateBinding
type clientStateBinding struct {
	client *Client
	item   *clientBindingTimeoutItem
}

func newClientStateBinding(c *Client) *clientStateBinding {
	s := &clientStateBinding{
		client: c,
	}
	s.item = &clientBindingTimeoutItem{c}
	// 等待binding超时
	serverInst.waitClientBinding(s.item)
	return s
}

func (s *clientStateBinding) onBind() {
	s.client.close()
}
func (s *clientStateBinding) onBindSuccess() {
	// 取消等待binding超时
	serverInst.stopWaitClientBinding(s.item)

	s.client.setState(newClientStateBinded(s.client))
}
func (s *clientStateBinding) onTimeout() {
	s.client.close()
}
func (s *clientStateBinding) onNewMessageToPlayer() {}

// clientStateBinded
type clientStateBinded struct {
	client *Client
	item   *clientBindedTimeoutItem
}

func newClientStateBinded(c *Client) *clientStateBinded {
	s := &clientStateBinded{
		client: c,
	}
	s.item = &clientBindedTimeoutItem{c}
	// 等待“长时间未收到客户端消息”的情况
	serverInst.waitClientTimeout(s.item)
	return s
}

func (s *clientStateBinded) onBind() {
	// 取消等待“长时间未收到客户端消息”的情况
	serverInst.stopWaitClientTimeout(s.item)

	s.client.close()
}
func (s *clientStateBinded) onBindSuccess() {}
func (s *clientStateBinded) onTimeout() {
	s.client.close()
}
func (s *clientStateBinded) onNewMessageToPlayer() {
	// 等待“长时间未收到客户端消息”的情况(更新timingwheel)。
	serverInst.waitClientTimeout(s.item)
}
