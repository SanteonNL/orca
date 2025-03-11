import { NextRequest, NextResponse } from 'next/server';
import { OperationOutcome, Bundle, Resource, SearchParameter, BundleEntry, FhirResource } from 'fhir/r4';
import { v4 } from 'uuid';
import { JSONPath } from 'jsonpath-plus';

/*
 * A very lightweight in-memory FHIR server that supports creating and reading resources of configured types.
 * It does not support updating or deleting resources. It also ONLY allows searching on SearchParameters
 * added directly into the code. 
 * 
 * Does NOT support vread, history, or any other FHIR operations.
 */
const supportedResourceTypes = ['Task', 'Condition', 'Observation'];
const resources: { [key: string]: Resource[] } = {
    Task: [],
    Condition: [],
    Observation: []
};

//expression MUST be json path
const searchParameters: SearchParameter[] = [{
    resourceType: 'SearchParameter',
    id: 'based-on',
    name: 'based-on',
    code: 'basedOn',
    type: 'reference',
    expression: '$.basedOn[*].identifier',
    base: [],
    description: '',
    status: 'active',
    url: ''
}];

export async function GET(req: NextRequest, { params }: { params: Promise<{ fhirPath: string }> }) {
    try {
        const { fhirPath } = await params;
        const fhirPathUrlSegment = Array.isArray(fhirPath) ? fhirPath.join('/') : fhirPath;
        const [resourceType, resourceId] = fhirPathUrlSegment.split('/');

        if (!supportedResourceTypes.includes(resourceType)) {
            const error: OperationOutcome = {
                resourceType: 'OperationOutcome',
                issue: [{
                    severity: 'error',
                    code: 'not-supported',
                    diagnostics: `Resource type ${resourceType} is not supported by this server. Supported resource types: ${supportedResourceTypes.join(', ')}`
                }]
            };
            return NextResponse.json(error, { status: 400 });
        }

        const searchParams = req.nextUrl.searchParams;

        const errorResponse = validateSearchParamKeys(searchParams);
        if (errorResponse) return errorResponse;

        const filteredResources = filterParams(resources[resourceType], searchParams);

        if (resourceId) return getResourceById(filteredResources, resourceId);

        const bundleEntries = filteredResources.map(resource => ({
            fullUrl: `${req.url}/${resource.id}`,
            resource: resource as FhirResource
        })) as BundleEntry<FhirResource>[];

        const result: Bundle = {
            resourceType: 'Bundle',
            type: 'searchset',
            total: filteredResources.length,
            entry: bundleEntries
        };

        return NextResponse.json(result, { status: 200 });
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        return NextResponse.json({ error: errorMessage }, { status: 500 });
    }
}

export async function POST(req: NextRequest) {
    try {
        const resource: Resource = await req.json();
        if (!supportedResourceTypes.includes(resource.resourceType)) {
            const error: OperationOutcome = {
                resourceType: 'OperationOutcome',
                issue: [{
                    severity: 'error',
                    code: 'not-supported',
                    diagnostics: `Resource type ${resource.resourceType} is not supported by this server. Supported resource types: ${supportedResourceTypes.join(', ')}`
                }]
            };
            return NextResponse.json(error, { status: 400 });
        }

        resource.id = v4();
        resources[resource.resourceType].push(resource);
        return NextResponse.json(resource, { status: 201 });
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        return NextResponse.json({ error: errorMessage }, { status: 500 });
    }
}

function validateSearchParamKeys(params: URLSearchParams) {

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
function filterParams(resources: Resource[], searchParams: URLSearchParams) {

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

function getResourceById(resources: Resource[], resourceId: string) {
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