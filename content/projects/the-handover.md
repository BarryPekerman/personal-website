# the-handover

https://play.barrypekerman.com

A browser-based capture-the-flag game -- inspired by OverTheWire's Bandit --
that hands each player a real, isolated Linux shell to hack through, not a
simulation.

PROBLEM
-------
A CTF teaches Linux by making players earn each level: find a secret in the
filesystem, use it to unlock the next stage. But that only works on a *real*
shell, and letting strangers run arbitrary commands on your infrastructure is
the textbook way to get owned. The whole game depends on isolation being strong
enough that it doesn't matter what a player types.

BUILD
-----
- One network-less Incus container per player: a real shell, with no route to
  the network, to other players, or to the host
- A capacity model instead of unbounded spawning -- a fixed pool of seats
  (currently 20), with live occupancy exposed at /healthz
- A landing page that gates entry to the game rather than dropping visitors
  straight onto a live container

RESULT
------
A running, publicly reachable CTF where the "sandbox" is a real OS per player,
not a trick -- players learn Linux by actually breaking through it. This is the
same isolated-shell system that the portfolio terminal's future "real shell"
mode was going to build; it already exists here.

STACK: Incus (system containers), Linux, network isolation, HTTP health/seat API
TODO (Barry): add a headline number if there is one -- levels, sessions served,
or uptime.
