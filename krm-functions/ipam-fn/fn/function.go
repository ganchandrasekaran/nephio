/*
 Copyright 2023 The Nephio Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package fn

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/nephio-project/nephio/krm-functions/lib/condkptsdk"
	"github.com/nephio-project/nephio/krm-functions/lib/kubeobject"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	corev1 "k8s.io/api/core/v1"
)

type FnR struct {
	ClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]
}

func (f *FnR) Run(rl *fn.ResourceList) (bool, error) {
	sdk, err := condkptsdk.New(
		rl,
		&condkptsdk.Config{
			For: corev1.ObjectReference{
				APIVersion: ipamv1alpha1.GroupVersion.Identifier(),
				Kind:       ipamv1alpha1.IPClaimKind,
			},
			PopulateOwnResourcesFn: nil,
			UpdateResourceFn:       f.updateIPClaimResource,
		},
	)
	if err != nil {
		rl.Results.ErrorE(err)
		return false, err
	}
	return sdk.Run()
}

// updateIPClaimResource provides an ip claim for a given KRM resource
// in the package by calling the ipam backend
func (f *FnR) updateIPClaimResource(forObj *fn.KubeObject, objs fn.KubeObjects) (*fn.KubeObject, error) {
	if forObj == nil {
		return nil, fmt.Errorf("expected a for object but got nil")
	}
	fn.Logf("ipclaim: %v\n", forObj)
	claimKOE, err := kubeobject.NewFromKubeObject[ipamv1alpha1.IPClaim](forObj)
	if err != nil {
		return nil, err
	}
	claim, err := claimKOE.GetGoStruct()
	if err != nil {
		return nil, err
	}
	newclaim := claim.DeepCopy()
	newclaim.Name = getNewName(claim.GetAnnotations(), claim.GetName())
	fn.Logf("ipclaim newName: %s\n", newclaim.Name)
	resp, err := f.ClientProxy.Claim(context.Background(), newclaim, nil)
	if err != nil {
		return nil, err
	}
	claim.Status = resp.Status

	if claim.Status.Prefix != nil {
		fn.Logf("claim resp prefix: %v\n", *resp.Status.Prefix)
	}
	if claim.Status.Gateway != nil {
		fn.Logf("claim resp gateway: %v\n", *resp.Status.Gateway)
	}
	// set the status
	err = claimKOE.SetStatus(resp)
	return &claimKOE.KubeObject, err
}

func getNewName(annotations map[string]string, origName string) string {
	split := strings.Split(annotations[condkptsdk.SpecializerPurpose], ".")
	return fmt.Sprintf("%s-%s", split[len(split)-1], origName)
}
