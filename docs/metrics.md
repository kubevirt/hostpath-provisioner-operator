# Hostpath Provisioner Operator Metrics

| Name | Kind | Type | Description |
|------|------|------|-------------|
| kubevirt_hpp_cr_ready | Metric | Gauge | HPP CR Ready |
| cluster:kubevirt_hpp_operator_up:sum | Recording rule | Gauge | The number of hostpath-provisioner-operator pods that are up |
| kubevirt_hpp_operator_up | Recording rule | Gauge | [Deprecated] The number of running hostpath-provisioner-operator pods |

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
