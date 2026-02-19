## `golden/` rules

Every supported resolver **must** consist of the standard set of golden rules, **as well as** conformance rules that reflect the degree to which it satisfies `kube-state-metrics`' Custom Resource State feature-set.

This should ideally be 100%, the lack of which should be followed with a review of the resolver implementation and whether it truly is unable to support the expected use-cases.

If so, the shortcomings must be clearly documented.

Below is the exhaustive list of golden rules that each resolver should, to the best of its ability, try to implement:

* TODO
