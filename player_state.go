package main

type playerState interface {
	onBind()
	onBindSuccess()

	onKick()
	onKickSuccess()

	onUnload()

	isOnline() bool
	onHeartbeat()
}

// playerStateOffline
type playerStateOffline struct {
	player *Player
	item   *playerUnloadItem
}

func newPlayerStateOffline(p *Player) *playerStateOffline {
	s := &playerStateOffline{
		player: p,
	}
	s.item = &playerUnloadItem{p}
	// 加入到检测unload的timingwheel中
	serverInst.waitToUnload(s.item)
	return s
}

func (s *playerStateOffline) onBind() {
	// 从检测unload的timingwheel中移除
	serverInst.stopUnload(s.item)

	s.player.setState(newPlayerStateBinding(s.player))
}
func (s *playerStateOffline) onBindSuccess() {}

func (s *playerStateOffline) onKick()        {}
func (s *playerStateOffline) onKickSuccess() {}

func (s *playerStateOffline) onUnload() {
	s.player.setState(newPlayerStateUnloading(s.player))
}

func (s *playerStateOffline) isOnline() bool {
	return false
}

func (s *playerStateOffline) onHeartbeat() {}

// playerStateBinding
type playerStateBinding struct {
	player *Player
}

func newPlayerStateBinding(p *Player) *playerStateBinding {
	return &playerStateBinding{
		player: p,
	}
}

func (s *playerStateBinding) onBind() {}
func (s *playerStateBinding) onBindSuccess() {
	s.player.setState(newPlayerStateOnline(s.player))
}

func (s *playerStateBinding) onKick()        {}
func (s *playerStateBinding) onKickSuccess() {}

func (s *playerStateBinding) onUnload() {}

func (s *playerStateBinding) isOnline() bool {
	return false
}

func (s *playerStateBinding) onHeartbeat() {}

// playerStateOnline
type playerStateOnline struct {
	player   *Player
	kickItem *playerKickItem
}

func newPlayerStateOnline(p *Player) *playerStateOnline {
	s := &playerStateOnline{
		player: p,
	}
	s.player.notifyBindSuccess()
	s.kickItem = &playerKickItem{p}
	// 加入到检测心跳包超时的timingwheel中
	serverInst.addPlayerToKick(s.kickItem)
	return s
}

func (s *playerStateOnline) onBind() {
	// 从检测心跳包超时的timingwheel中移除
	serverInst.stopKick(s.kickItem)

	s.player.setState(newPlayerStateBinding(s.player))
}
func (s *playerStateOnline) onBindSuccess() {}

func (s *playerStateOnline) onKick() {
	s.player.setState(newPlayerStateKicking(s.player))
}
func (s *playerStateOnline) onKickSuccess() {}

func (s *playerStateOnline) onUnload() {}

func (s *playerStateOnline) isOnline() bool {
	return true
}

func (s *playerStateOnline) onHeartbeat() {
	// 更新检测心跳包超时的timingwheel中的记录
	serverInst.addPlayerToKick(s.kickItem)
}

// playerStateKicking
type playerStateKicking struct {
	player *Player
}

func newPlayerStateKicking(p *Player) *playerStateKicking {
	s := &playerStateKicking{
		player: p,
	}
	serverInst.kickPlayer(s.player.uid())
	return s
}

func (s *playerStateKicking) onBind()        {}
func (s *playerStateKicking) onBindSuccess() {}

func (s *playerStateKicking) onKick() {}
func (s *playerStateKicking) onKickSuccess() {
	s.player.setState(newPlayerStateOffline(s.player))
}

func (s *playerStateKicking) onUnload() {}

func (s *playerStateKicking) isOnline() bool {
	return false
}

func (s *playerStateKicking) onHeartbeat() {}

// playerStateUnloading
type playerStateUnloading struct {
	player *Player
}

func newPlayerStateUnloading(p *Player) *playerStateUnloading {
	s := &playerStateUnloading{
		player: p,
	}
	serverInst.removePlayer(s.player)
	s.player.notifyUnload()
	return s
}

func (s *playerStateUnloading) onBind()        {}
func (s *playerStateUnloading) onBindSuccess() {}

func (s *playerStateUnloading) onKick()        {}
func (s *playerStateUnloading) onKickSuccess() {}

func (s *playerStateUnloading) onUnload() {}

func (s *playerStateUnloading) isOnline() bool {
	return false
}

func (s *playerStateUnloading) onHeartbeat() {}
