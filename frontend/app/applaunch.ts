import {Endpoint, Bundle, Identifier} from 'fhir/r4';
import {getPatientViewerTestUrl} from "@/app/actions";
import Client from "fhir-kit-client";

export interface LaunchableApp {
    Name: string;
    URL: string;
}

export async function getLaunchableApps(scpClient: Client, organization: Identifier) : Promise<LaunchableApp[]> {
    const testAppURL = await getPatientViewerTestUrl();
    if (testAppURL) {
        return [{
            Name: "Test App",
            URL: testAppURL,
        }]
    }
    let endpoints = await scpClient.search({
        resourceType: "Endpoint",
        headers: {
            "Cache-Control": "no-cache",
            "X-Scp-Entity-Identifier": `${organization.system}|${organization.value}`,
        }
    }) as Bundle<Endpoint>;
    // filter endpoints, only show endpoints that:
    // - have status 'active';
    // - have connection type `http://santeonnl.github.io/shared-care-planning/endpoint-connection-type|web-oauth2`
    return endpoints.entry?.filter((entry) => {
        if (!entry.resource) {
            return false
        }
        const endpoint = entry?.resource as Endpoint;
        return endpoint.status == "active"
            && endpoint.connectionType?.system == "http://santeonnl.github.io/shared-care-planning/endpoint-connection-type"
            && endpoint.connectionType?.code == "web-oauth2"
            && endpoint.name;
    }).map((entry) => {
        const endpoint = entry?.resource as Endpoint;
        return {
            Name: endpoint.name!,
            URL: endpoint.address,
        }
    }) ?? []
}