package framework

import (
	"time"
)

type chainTime struct {
	genesisTime    time.Time
	secondsPerSlot uint64
	slotsPerEpoch  uint64
	ResCh          chan SlotResult
	CloseCh        chan struct{}
	ReadyCh        chan struct{}
	startEpoch     uint64
}

func NewChainTime(genesisTime time.Time, secondsPerSlot, slotsPerEpoch uint64) *chainTime {
	return &chainTime{
		genesisTime:    genesisTime,
		secondsPerSlot: secondsPerSlot,
		slotsPerEpoch:  slotsPerEpoch,
		ReadyCh:        make(chan struct{}),
		CloseCh:        make(chan struct{}),
		ResCh:          make(chan SlotResult),
	}
}

type SlotResult struct {
	Epoch          uint64
	StartTime      time.Time
	GenesisTime    time.Time
	SecondsPerSlot uint64

	FirstSlot uint64
	LastSlot  uint64
}

func (s *SlotResult) AtSlot(slot uint64) time.Time {
	return s.GenesisTime.Add(time.Duration(slot*s.SecondsPerSlot) * time.Second)
}

func (s *SlotResult) AtSlotAndStage(slot uint64) time.Time {
	return s.GenesisTime.Add(time.Duration(slot*s.SecondsPerSlot) * time.Second)
}

func (b *chainTime) Run() {
	secondsPerEpoch := time.Duration(b.secondsPerSlot*b.slotsPerEpoch) * time.Second

	// time since genesis
	currentTime := time.Now()
	timeSinceGenesis := currentTime.Sub(b.genesisTime)

	if timeSinceGenesis < 0 {
		// wait until the chain has started
		timeUntilGenesis := b.genesisTime.Sub(currentTime)
		select {
		case <-time.After(timeUntilGenesis):
			timeSinceGenesis = 0

		case <-b.CloseCh:
			return
		}
	}

	nextTick := timeSinceGenesis.Truncate(secondsPerEpoch) + secondsPerEpoch
	epoch := uint64(nextTick / secondsPerEpoch)
	nextTickTime := b.genesisTime.Add(nextTick)

	b.startEpoch = epoch

	// close the ready channel to notify that the
	// chain has started
	close(b.ReadyCh)

	emitEpoch := func(epoch uint64) {
		startTime := b.genesisTime.Add(time.Duration(epoch*b.slotsPerEpoch*b.secondsPerSlot) * time.Second)

		firstSlot := epoch * b.slotsPerEpoch
		lastSlot := epoch*b.slotsPerEpoch + b.slotsPerEpoch

		b.ResCh <- SlotResult{
			Epoch:          epoch,
			StartTime:      startTime,
			SecondsPerSlot: b.secondsPerSlot,
			GenesisTime:    b.genesisTime,
			FirstSlot:      firstSlot,
			LastSlot:       lastSlot,
		}
	}

	emitEpoch(epoch - 1)

	for {
		timeToWait := nextTickTime.Sub(time.Now())

		select {
		case <-time.After(timeToWait):
		case <-b.CloseCh:
			return
		}

		emitEpoch(epoch)
		epoch++
		nextTickTime = nextTickTime.Add(secondsPerEpoch)
	}
}
