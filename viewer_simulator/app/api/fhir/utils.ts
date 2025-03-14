import { OperationOutcome, Resource } from "fhir/r4";
import { searchParameters } from "./config";
import { NextResponse } from "next/server";
import { JSONPath } from "jsonpath-plus";

export function validateSearchParamKeys(params: URLSearchParams) {

    for (const param of Array.from(params.keys())) {
        if (!searchParameters.some(sp => sp.code === param)) {
            const error: OperationOutcome = {
                resourceType: 'OperationOutcome',
                issue: [{
                    severity: 'error',
                    code: 'not-supported',
                    diagnostics: `Search parameter ${param} is not supported by this server. Supported search parameters: ${searchParameters.map(sp => sp.code).join(', ')}`
                }]
            };
            return NextResponse.json(error, { status: 400 });
        }
    }
}

/*
* Filter resources based on search parameters. This is a VERY basic implementation of search parameters.
* It only does an exact match on a JSON expression, or a system|code match on a coding. 
*/
export function filterParams(resources: Resource[], searchParams: URLSearchParams) {

    if (!searchParams.size) return resources;

    for (const paramKey of Array.from(searchParams.keys())) {
        const paramValue = searchParams.get(paramKey);
        const searchParam = searchParameters.find(sp => sp.code === paramKey);

        if (searchParam && searchParam.expression) {
            return resources.filter(resource => {
                const result = JSONPath({ path: searchParam.expression!, json: resource });
                if (result.length > 0) {

                    //check for a coding
                    if (result[0].system && result[0].value) {
                        if (!paramValue) return false;
                        const [system, code] = paramValue.includes('|') ? paramValue.split('|') : [undefined, paramValue];

                        return result.some((item: any) => {
                            if (system) {
                                return item.system === system && item.value === code;
                            }
                            return item.value === code;
                        });

                    } else {
                        return result.some((item: any) => item.code === paramValue);
                    }
                }
                return false;
            });
        }
    }

    return [];
}

export function getResourceById(resources: Resource[], resourceId: string) {
    const resource = resources.find(r => r.id === resourceId);
    if (!resource) {
        const error: OperationOutcome = {
            resourceType: 'OperationOutcome',
            issue: [{
                severity: 'error',
                code: 'not-found',
                diagnostics: `Resource with id ${resourceId} not found`
            }]
        };
        return NextResponse.json(error, { status: 404 });
    }

    return NextResponse.json(resource, { status: 200 });
}