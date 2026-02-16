## `/external`: Why do we need this?

While RSM tries its best to fulfill all of its expected use-cases through the resolver framework, there may be times where teams might want a little more "turing-ness", which none of the non-turing-complete domain-specific-languages in the resolver feature-set may offer at that time.

To circumvent such instances, RSM offers an `/external` endpoint. There are two ways to utilize this as of now:
* Contributing to this directory directly: Any collector that is:
  * not suited for KSM under the guidelines levied by it, and,
  * has enough community consensus backing it to establish a reasonable need for it across the ecosystem unequivocally.
* Contributing to this directory in your downstream fork: Any collector that teams in your organization deem worthy of having, but their implementation through managed resources (`ResourceMetricMonitors`s) is limited or not possible altogether. Please do keep in mind that, wherever possible, managed resources should be used to implement collectors instead. As such, it is also recommended for such implementations to be reviewed from time to time, to see if they are supported by the resolver framework in the future, at which point they can be moved out from the binary to managed resources under `namespace`s owned by appropriate teams, for separation of concerns and isolation, above everything else.

Please refer to [the custom collector example](./clusterresourcequota.go.md) to know more on how to add collectors here.

#### TODO

- [ ] Support enabling collectors here on a case-by-case basis (opt-in flags).
