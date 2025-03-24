import { NextRequest, NextResponse } from 'next/server';
import { OperationOutcome, Bundle, Resource, SearchParameter, BundleEntry, FhirResource } from 'fhir/r4';
import { v4 } from 'uuid';
import { resources, supportedResourceTypes } from '../config';
import { filterParams, getResourceById, validateSearchParamKeys } from '../utils';
import { FHIR_SCP_CONTEXT_SYSTEM } from "@/utils/const/const";

/*
 * A very lightweight in-memory FHIR server that supports creating and reading resources of configured types.
 * It does not support updating or deleting resources. It also ONLY allows searching on SearchParameters
 * added directly into the code. 
 * 
 * The fhir server also has minor SCP logic embedded. When a request is made with the X-Scp-Context header, it
 * adds the basedOn search parameter to the search query. This is to simulate the SCP context in the FHIR server.
 * The basedOn doesn't always exist on resources, but currently we're only using it to make Observations. If needed
 * create a mapping between resource type and search param
 * 
 * 
 * Does NOT support vread, history, or any other FHIR operations.
 */

export async function GET(req: NextRequest, { params }: { params: Promise<{ fhirPath: string }> }) {
    try {
        const { fhirPath } = await params;
        const fhirPathUrlSegment = Array.isArray(fhirPath) ? fhirPath.join('/') : fhirPath;
        const [resourceType, resourceId] = fhirPathUrlSegment.replace("/_search", "").split('/');

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

        const scpContext = req.headers.get("X-Scp-Context");
        if (scpContext) {
            console.log("SCP Request - Adding basedOn search parameter for SCP context: ", scpContext);
            searchParams.set("basedOn", `${FHIR_SCP_CONTEXT_SYSTEM}|${scpContext}`);
        }
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

export async function POST(req: NextRequest, { params }: { params: Promise<{ fhirPath: string }> }) {
    try {

        const { fhirPath } = await params;
        if (fhirPath[fhirPath.length - 1] === '_search') {
            return GET(req, { params });
        }

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