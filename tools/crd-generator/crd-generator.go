package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

func main() {

	filename := flag.String("sourcefile", "deploy/operator.yaml", "crd source file")
	outputdir := flag.String("outputDir", "tools/helper", "path to dir where go file will be generated")

	flag.Parse()
	fileInfo, err := os.Stat(*filename)
	if err != nil {
		panic(fmt.Errorf("failed to stat file %v, %v", filename, err))
	}
	file, err := os.Open(*filename)
	if err != nil {
		panic(fmt.Errorf("failed to read file %v, %v", filename, err))
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	fileScanner := bufio.NewScanner(file)
	buf := make([]byte, fileInfo.Size())
	fileScanner.Buffer(buf, len(buf))
	searchBytes := []byte("---")
    searchLen := len(searchBytes)
	fileScanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
        dataLen := len(data)

        // Return nothing if at end of file and no data passed
        if atEOF && dataLen == 0 {
            return 0, nil, nil
        }

        // Find next separator and return token
        if i := bytes.Index(data, searchBytes); i >= 0 {
            return i + searchLen, data[0:i], nil
        }

        // If we're at EOF, we have a final, non-terminated line. Return it.
        if atEOF {
            return dataLen, data, nil
        }

        // Request more data.
        return
    })
	for fileScanner.Scan() {
		item := fileScanner.Bytes()
		crdName, crd := getCRD(item)
		if crd != nil && crdName != "" {
			fmt.Printf("Generating CRD %s\n", crdName)
			generateCrdGoFile(*outputdir, crd)
		}
		clusterRoleName, clusterRole := getClusterRoles(item)
		if clusterRole != nil && clusterRoleName != "" && len(clusterRole.Rules) > 0 && clusterRole.Kind == "ClusterRole" {
			fmt.Printf("Generating cluster role %s\n", clusterRoleName)
			generateClusterRoleGoFile(*outputdir, clusterRole)
		}
		roleName, role := getRoles(item)
		if role != nil && roleName != "" && len(role.Rules) > 0 && role.Kind == "Role" {
			fmt.Printf("Generating role %s\n", clusterRoleName)
			generateRoleGoFile(*outputdir, role)
		}
		deploymentName, operatorDeployment := getOperatorDeployment(item)
		if operatorDeployment != nil && deploymentName != "" && operatorDeployment.Kind == "Deployment" {
			fmt.Printf("Generating operator deployment %s\n", deploymentName)
			generateDeploymentGoFile(*outputdir, operatorDeployment)
		}
	}

}

func generateCrdGoFile(outputDir string, crd *extv1.CustomResourceDefinition) {
	filepath := filepath.Join(outputDir, "crd_generated.go")
	os.Remove(filepath)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	if err != nil {
		panic(fmt.Errorf("failed to create go file %v, %v", filepath, err))
	}
	fmt.Printf("output file: %s\n", file.Name())

	file.WriteString("package helper\n\n")
	file.WriteString("//hppCRD is a string yaml of the hpp CRD\n")
	file.WriteString("var hppCRD string = \n`")

	crd.Status = extv1.CustomResourceDefinitionStatus{}
	b, _ := yaml.Marshal(crd)
	file.WriteString(string(b))
	file.WriteString("`\n")

}

func generateClusterRoleGoFile(outputDir string, clusterRole *rbacv1.ClusterRole) {
	filepath := filepath.Join(outputDir, "cluster_role_generated.go")
	os.Remove(filepath)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	if err != nil {
		panic(fmt.Errorf("failed to create go file %v, %v", filepath, err))
	}
	fmt.Printf("output file: %s\n", file.Name())

	file.WriteString("package helper\n\n")
	file.WriteString("//HppOperatorClusterRole is a string yaml of the hpp operator cluster role\n")
	file.WriteString("var HppOperatorClusterRole string = \n`")

	b, _ := yaml.Marshal(clusterRole)
	file.WriteString(string(b))
	file.WriteString("`\n")

}

func generateRoleGoFile(outputDir string, role *rbacv1.Role) {
	filepath := filepath.Join(outputDir, "role_generated.go")
	os.Remove(filepath)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	if err != nil {
		panic(fmt.Errorf("failed to create go file %v, %v", filepath, err))
	}
	fmt.Printf("output file: %s\n", file.Name())

	file.WriteString("package helper\n\n")
	file.WriteString("//HppOperatorRole is a string yaml of the hpp operator role\n")
	file.WriteString("var HppOperatorRole string = \n`")

	b, _ := yaml.Marshal(role)
	file.WriteString(string(b))
	file.WriteString("`\n")
}

func generateDeploymentGoFile(outputDir string, deployment *appsv1.Deployment) {
	filepath := filepath.Join(outputDir, "operator_deployment_generated.go")
	os.Remove(filepath)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	if err != nil {
		panic(fmt.Errorf("failed to create go file %v, %v", filepath, err))
	}
	fmt.Printf("output file: %s\n", file.Name())

	file.WriteString("package helper\n\n")
	file.WriteString("//HppOperatorDeployment is a string yaml of the hpp operator deployment\n")
	file.WriteString("var HppOperatorDeployment string = \n`")

	b, _ := yaml.Marshal(deployment)
	file.WriteString(string(b))
	file.WriteString("`\n")
}


func getCRD(text []byte) (string, *extv1.CustomResourceDefinition) {
	crd := extv1.CustomResourceDefinition{}
	err := k8syaml.NewYAMLToJSONDecoder(bytes.NewBuffer(text)).Decode(&crd)
	if err != nil {
		panic(fmt.Errorf("failed to parse crd from text %s, %v", string(text), err))
	}
	return crd.Spec.Names.Singular, &crd
}

func getClusterRoles(text []byte) (string, *rbacv1.ClusterRole) {
	clusterRole := rbacv1.ClusterRole{}
	err := k8syaml.NewYAMLToJSONDecoder(bytes.NewBuffer(text)).Decode(&clusterRole)
	if err != nil {
		panic(fmt.Errorf("failed to parse cluster role from text %s, %v", string(text), err))
	}
	return clusterRole.Name, &clusterRole
}

func getRoles(text []byte) (string, *rbacv1.Role) {
	role := rbacv1.Role{}
	err := k8syaml.NewYAMLToJSONDecoder(bytes.NewBuffer(text)).Decode(&role)
	if err != nil {
		panic(fmt.Errorf("failed to parse role from text %s, %v", string(text), err))
	}
	return role.Name, &role
}

func getOperatorDeployment(text []byte) (string, *appsv1.Deployment) {
	deployment := appsv1.Deployment{}
	err := k8syaml.NewYAMLToJSONDecoder(bytes.NewBuffer(text)).Decode(&deployment)
	if err != nil {
		panic(fmt.Errorf("failed to parse deployment from text %s, %v", string(text), err))
	}
	return deployment.Name, &deployment
}
