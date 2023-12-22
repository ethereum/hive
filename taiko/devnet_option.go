package taiko

func WithL1Node(l1 *ELNode) DevOption {
	return func(d *Devnet) {
		d.Lock()
		defer d.Unlock()
		d.L1Engines = append(d.L1Engines, l1)
	}
}

func WithL2Node(l2 *ELNode) DevOption {
	return func(d *Devnet) {
		d.Lock()
		defer d.Unlock()
		d.L2Engines = append(d.L2Engines, l2)
	}
}

func WithDriverNode(n *Node) DevOption {
	return func(d *Devnet) {
		d.Lock()
		defer d.Unlock()
		d.drivers = append(d.drivers, n)
	}
}

func WithProposerNode(n *Node) DevOption {
	return func(d *Devnet) {
		d.Lock()
		defer d.Unlock()
		d.proposers = append(d.proposers, n)
	}
}

func WithProverNode(n *Node) DevOption {
	return func(d *Devnet) {
		d.Lock()
		defer d.Unlock()
		d.provers = append(d.provers, n)
	}
}
