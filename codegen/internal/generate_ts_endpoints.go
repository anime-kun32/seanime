package codegen

import (
	"fmt"
	"github.com/samber/lo"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	typescriptEndpointsFileName     = "endpoints.ts"
	typescriptEndpointTypesFileName = "endpoint.types.ts"
	typescriptHooksFileName         = "hooks_template.ts"
	space                           = "    "
)

var additionalStructNamesForEndpoints = []string{
	"vendor_hibike_torrent.AnimeTorrent",
}

func GenerateTypescriptEndpointsFile(docsPath string, structsPath string, outDir string) []string {
	handlers := LoadHandlers(docsPath)
	structs := LoadPublicStructs(structsPath)

	_ = os.MkdirAll(outDir, os.ModePerm)
	f, err := os.Create(filepath.Join(outDir, typescriptEndpointsFileName))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	typeF, err := os.Create(filepath.Join(outDir, typescriptEndpointTypesFileName))
	if err != nil {
		panic(err)
	}
	defer typeF.Close()

	hooksF, err := os.Create(filepath.Join(outDir, typescriptHooksFileName))
	if err != nil {
		panic(err)
	}
	defer hooksF.Close()

	f.WriteString("// This code was generated by codegen/main.go. DO NOT EDIT.\n\n")

	f.WriteString(`export type ApiEndpoints = Record<string, Record<string, {
     key: string,
     methods: ("POST" | "GET" | "PATCH" | "PUT" | "DELETE")[],
     endpoint: string
}>>

`)

	f.WriteString("export const API_ENDPOINTS = {\n")

	groupedByFile := make(map[string][]*RouteHandler)
	for _, handler := range handlers {
		if _, ok := groupedByFile[handler.Filename]; !ok {
			groupedByFile[handler.Filename] = make([]*RouteHandler, 0)
		}
		groupedByFile[handler.Filename] = append(groupedByFile[handler.Filename], handler)
	}

	filenames := make([]string, 0)
	for k := range groupedByFile {
		filenames = append(filenames, k)
	}

	slices.SortStableFunc(filenames, func(i, j string) int {
		return strings.Compare(i, j)
	})

	for _, filename := range filenames {
		routes := groupedByFile[filename]
		if len(routes) == 0 {
			continue
		}

		if lo.EveryBy(routes, func(route *RouteHandler) bool {
			return route.Api == nil || len(route.Api.Methods) == 0
		}) {
			continue
		}

		groupName := strings.ToUpper(strings.TrimSuffix(filename, ".go"))

		writeLine(f, fmt.Sprintf("\t%s: {", groupName)) // USERS: {

		for _, route := range groupedByFile[filename] {
			if route.Api == nil || len(route.Api.Methods) == 0 {
				continue
			}

			if len(route.Api.Descriptions) > 0 {
				writeLine(f, "        /**")
				f.WriteString(fmt.Sprintf("         *  @description\n"))
				f.WriteString(fmt.Sprintf("         *  Route %s\n", route.Api.Summary))
				for _, cmt := range route.Api.Descriptions {
					writeLine(f, fmt.Sprintf("         *  %s", strings.TrimSpace(cmt)))
				}
				writeLine(f, "         */")
			}

			writeLine(f, fmt.Sprintf("\t\t%s: {", strings.TrimPrefix(route.Name, "Handle"))) // GetAnimeCollection: {

			methodStr := ""
			if len(route.Api.Methods) > 1 {
				methodStr = fmt.Sprintf("\"%s\"", strings.Join(route.Api.Methods, "\", \""))
			} else {
				methodStr = fmt.Sprintf("\"%s\"", route.Api.Methods[0])
			}

			writeLine(f, fmt.Sprintf("\t\t\tkey: \"%s\",", getEndpointKey(route.Name, groupName)))

			writeLine(f, fmt.Sprintf("\t\t\tmethods: [%s],", methodStr)) // methods: ['GET'],

			writeLine(f, fmt.Sprintf("\t\t\tendpoint: \"%s\",", route.Api.Endpoint)) // path: '/api/v1/anilist/collection',

			writeLine(f, "\t\t},") // },
		}

		writeLine(f, "\t},") // },
	}

	f.WriteString("} satisfies ApiEndpoints\n\n")

	referenceGoStructs := make([]string, 0)
	for _, filename := range filenames {
		routes := groupedByFile[filename]
		if len(routes) == 0 {
			continue
		}
		for _, route := range groupedByFile[filename] {
			if route.Api == nil || len(route.Api.Methods) == 0 {
				continue
			}
			if len(route.Api.Params) == 0 && len(route.Api.BodyFields) == 0 {
				continue
			}
			for _, param := range route.Api.BodyFields {
				if param.UsedStructType != "" {
					referenceGoStructs = append(referenceGoStructs, param.UsedStructType)
				}
			}
			for _, param := range route.Api.Params {
				if param.UsedStructType != "" {
					referenceGoStructs = append(referenceGoStructs, param.UsedStructType)
				}
			}
		}
	}
	referenceGoStructs = lo.Uniq(referenceGoStructs)

	typeF.WriteString("// This code was generated by codegen/main.go. DO NOT EDIT.\n\n")

	//
	// Imports
	//
	importedTypes := make([]string, 0)
	//
	for _, structName := range referenceGoStructs {
		parts := strings.Split(structName, ".")
		if len(parts) != 2 {
			continue
		}

		var goStruct *GoStruct
		for _, s := range structs {
			if s.Name == parts[1] && s.Package == parts[0] {
				goStruct = s
				break
			}
		}

		if goStruct == nil {
			continue
		}

		importedTypes = append(importedTypes, goStruct.FormattedName)
	}

	for _, otherStrctName := range additionalStructNamesForEndpoints {
		importedTypes = append(importedTypes, stringGoTypeToTypescriptType(otherStrctName))
	}
	//
	slices.SortStableFunc(importedTypes, func(i, j string) int {
		return strings.Compare(i, j)
	})
	typeF.WriteString("import type {\n")
	for _, typeName := range importedTypes {
		typeF.WriteString(fmt.Sprintf("    %s,\n", typeName))
	}
	typeF.WriteString("} from \"@/api/generated/types.ts\"\n\n")

	//
	// Types
	//

	for _, filename := range filenames {
		routes := groupedByFile[filename]
		if len(routes) == 0 {
			continue
		}

		typeF.WriteString("//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////\n")
		typeF.WriteString(fmt.Sprintf("// %s\n", strings.TrimSuffix(filename, ".go")))
		typeF.WriteString("//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////\n\n")

		for _, route := range groupedByFile[filename] {
			if route.Api == nil || len(route.Api.Methods) == 0 {
				continue
			}

			if len(route.Api.Params) == 0 && len(route.Api.BodyFields) == 0 {
				continue
			}

			typeF.WriteString("/**\n")
			typeF.WriteString(fmt.Sprintf(" * - Filepath: %s\n", filepath.ToSlash(strings.TrimPrefix(route.Filepath, "..\\"))))
			typeF.WriteString(fmt.Sprintf(" * - Filename: %s\n", route.Filename))
			typeF.WriteString(fmt.Sprintf(" * - Endpoint: %s\n", route.Api.Endpoint))
			if len(route.Api.Summary) > 0 {
				typeF.WriteString(fmt.Sprintf(" * @description\n"))
				typeF.WriteString(fmt.Sprintf(" * Route %s\n", strings.TrimSpace(route.Api.Summary)))
			}
			typeF.WriteString(" */\n")
			typeF.WriteString(fmt.Sprintf("export type %s_Variables = {\n", strings.TrimPrefix(route.Name, "Handle"))) // export type EditAnimeEntry_Variables = {

			addedBodyFields := false
			for _, param := range route.Api.BodyFields {
				writeParamField(typeF, route, param) // mediaId: number;
				if param.UsedStructType != "" {
					referenceGoStructs = append(referenceGoStructs, param.UsedStructType)
				}
				addedBodyFields = true
			}

			if !addedBodyFields {
				for _, param := range route.Api.Params {
					writeParamField(typeF, route, param) // mediaId: number;
					if param.UsedStructType != "" {
						referenceGoStructs = append(referenceGoStructs, param.UsedStructType)
					}
				}
			}

			writeLine(typeF, "}\n")
		}

	}

	generateHooksFile(hooksF, groupedByFile, filenames)

	return referenceGoStructs
}

func generateHooksFile(f *os.File, groupedHandlers map[string][]*RouteHandler, filenames []string) {

	queryTemplate := `// export function use{handlerName}({props}) {
//     return useServerQuery{<}{TData}{TVar}{>}({
//         endpoint: API_ENDPOINTS.{groupName}.{handlerName}.endpoint{endpointSuffix},
//         method: API_ENDPOINTS.{groupName}.{handlerName}.methods[%d],
//         queryKey: [API_ENDPOINTS.{groupName}.{handlerName}.key],
//         enabled: true,
//     })
// }

`
	mutationTemplate := `// export function use{handlerName}({props}) {
//     return useServerMutation{<}{TData}{TVar}{>}({
//         endpoint: API_ENDPOINTS.{groupName}.{handlerName}.endpoint{endpointSuffix},
//         method: API_ENDPOINTS.{groupName}.{handlerName}.methods[%d],
//         mutationKey: [API_ENDPOINTS.{groupName}.{handlerName}.key],
//         onSuccess: async () => {
// 
//         },
//     })
// }

`

	tmpGroupTmpls := make(map[string][]string)

	for _, filename := range filenames {
		routes := groupedHandlers[filename]
		if len(routes) == 0 {
			continue
		}

		if lo.EveryBy(routes, func(route *RouteHandler) bool {
			return route.Api == nil || len(route.Api.Methods) == 0
		}) {
			continue
		}

		f.WriteString("//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////\n")
		f.WriteString(fmt.Sprintf("// %s\n", strings.TrimSuffix(filename, ".go")))
		f.WriteString("//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////\n\n")

		tmpls := make([]string, 0)
		for _, route := range groupedHandlers[filename] {
			if route.Api == nil || len(route.Api.Methods) == 0 {
				continue
			}

			for i, method := range route.Api.Methods {

				tmpl := ""

				if method == "GET" {
					getTemplate := strings.ReplaceAll(queryTemplate, "{handlerName}", strings.TrimPrefix(route.Name, "Handle"))
					getTemplate = strings.ReplaceAll(getTemplate, "{groupName}", strings.ToUpper(strings.TrimSuffix(filename, ".go")))
					getTemplate = strings.ReplaceAll(getTemplate, "{method}", "GET")
					tmpl = getTemplate
				}

				if method == "POST" || method == "PATCH" || method == "PUT" || method == "DELETE" {
					mutTemplate := strings.ReplaceAll(mutationTemplate, "{handlerName}", strings.TrimPrefix(route.Name, "Handle"))
					mutTemplate = strings.ReplaceAll(mutTemplate, "{groupName}", strings.ToUpper(strings.TrimSuffix(filename, ".go")))
					mutTemplate = strings.ReplaceAll(mutTemplate, "{method}", method)
					tmpl = mutTemplate
				}

				tmpl = strings.ReplaceAll(tmpl, "%d", strconv.Itoa(i))

				if len(route.Api.ReturnTypescriptType) == 0 {
					tmpl = strings.ReplaceAll(tmpl, "{<}", "")
					tmpl = strings.ReplaceAll(tmpl, "{TData}", "")
					tmpl = strings.ReplaceAll(tmpl, "{TVar}", "")
					tmpl = strings.ReplaceAll(tmpl, "{>}", "")
				} else {
					tmpl = strings.ReplaceAll(tmpl, "{<}", "<")
					tmpl = strings.ReplaceAll(tmpl, "{TData}", route.Api.ReturnTypescriptType)
					tmpl = strings.ReplaceAll(tmpl, "{>}", ">")
				}

				if len(route.Api.Params) == 0 {
					tmpl = strings.ReplaceAll(tmpl, "{endpointSuffix}", "")
					tmpl = strings.ReplaceAll(tmpl, "{props}", "")
				} else {
					props := ""
					for _, param := range route.Api.Params {
						props += fmt.Sprintf(`%s: %s, `, param.JsonName, param.TypescriptType)
					}
					tmpl = strings.ReplaceAll(tmpl, "{props}", props[:len(props)-2])
					endpointSuffix := ""
					for _, param := range route.Api.Params {
						endpointSuffix += fmt.Sprintf(`.replace("{%s}", String(%s))`, param.JsonName, param.JsonName)
					}
					tmpl = strings.ReplaceAll(tmpl, "{endpointSuffix}", endpointSuffix)
				}

				if len(route.Api.BodyFields) == 0 {
					tmpl = strings.ReplaceAll(tmpl, "{TVar}", "")
				} else {
					tmpl = strings.ReplaceAll(tmpl, "{TVar}", fmt.Sprintf(", %s", strings.TrimPrefix(route.Name, "Handle")+"_Variables"))
				}

				tmpls = append(tmpls, tmpl)
				f.WriteString(tmpl)

			}

		}
		tmpGroupTmpls[strings.TrimSuffix(filename, ".go")] = tmpls
	}

	//for filename, tmpls := range tmpGroupTmpls {
	//	hooksF, err := os.Create(filepath.Join("../seanime-web/src/api/hooks", filename+".hooks.ts"))
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer hooksF.Close()
	//
	//	for _, tmpl := range tmpls {
	//		hooksF.WriteString(tmpl)
	//	}
	//}

}

func writeParamField(f *os.File, handler *RouteHandler, param *RouteHandlerParam) {
	if len(param.Descriptions) > 0 {
		writeLine(f, "\t/**")
		for _, cmt := range param.Descriptions {
			writeLine(f, fmt.Sprintf("\t *  %s", strings.TrimSpace(cmt)))
		}
		writeLine(f, "\t */")
	}
	fieldSuffix := ""
	if !param.Required {
		fieldSuffix = "?"
	}
	writeLine(f, fmt.Sprintf("\t%s%s: %s", param.JsonName, fieldSuffix, param.TypescriptType))
}

func getEndpointKey(s string, groupName string) string {
	s = strings.TrimPrefix(s, "Handle")
	var result string
	for i, v := range s {
		if i > 0 && v >= 'A' && v <= 'Z' {
			result += "-"
		}

		result += string(v)
	}
	result = strings.ToLower(result)
	if strings.Contains(result, "t-v-d-b") {
		result = strings.Replace(result, "t-v-d-b", "tvdb", 1)
	}
	if strings.Contains(result, "m-a-l") {
		result = strings.Replace(result, "m-a-l", "mal", 1)
	}
	return strings.ReplaceAll(groupName, "_", "-") + "-" + result
}

func writeLine(file *os.File, template string) {
	template = strings.ReplaceAll(template, "\t", space)
	file.WriteString(fmt.Sprintf(template + "\n"))
}
