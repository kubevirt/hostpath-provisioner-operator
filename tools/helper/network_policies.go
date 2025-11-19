package helper

import (
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"kubevirt.io/hostpath-provisioner-operator/pkg/controller/hostpathprovisioner"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	allowIngressToMetrics               = "hpp-allow-ingress-to-metrics"
	allowIngressToOperatorWebhookServer = "hpp-allow-ingress-to-operator-webhook-server"
)

func CreateNetworkPolicies(namespace string) []client.Object {
	return []client.Object{
		newIngressToMetricsNP(namespace),
		newIngressToOperatorWebhookServer(namespace),
	}
}

func newNetworkPolicy(namespace, name string, spec *networkv1.NetworkPolicySpec) *networkv1.NetworkPolicy {
	return &networkv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{hostpathprovisioner.MultiPurposeHostPathProvisionerName: ""},
		},
		Spec: *spec,
	}
}

func newIngressToMetricsNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowIngressToMetrics,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{hostpathprovisioner.PrometheusLabelKey: "true"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8080)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func newIngressToOperatorWebhookServer(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowIngressToOperatorWebhookServer,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": hostpathprovisioner.OperatorImageDefault},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(9443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}
