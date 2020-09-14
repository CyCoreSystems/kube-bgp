# Kube-BGP

Kube-BGP is a simple iBGP mesh wrapper for GoBGP which generates and maintains
the GoBGP configuration for the current state of the kubernetes cluster.

Nodes may be designated as route reflectors, in which case they will peer to
specified external BGP endpoints, reflecting all internal routes.

Kube-BGP is dual-stack capable, but in the event of an IPv6-only cluster, you
must manually supply the router-id annotation for each node.  You may
_optionally_ supply one for IPv4 or dual-stack clusters, in which case the
supplied router-id will be used instead of the auto-detected one.


