import Client from "fhir-kit-client";
import { ReplacePatch } from "fhir-kit-client/types/externals";
import { BundleEntry, Task } from "fhir/r4";
import { NextRequest, NextResponse } from "next/server";

let isPolling = false

export async function POST(req: NextRequest) {

    if (isPolling) return NextResponse.json({ status: "ok" })

    const body = await req.json()

    const baseUrl = body["fhir_base_url"]
    console.log(baseUrl)
    const fhirClient = new Client({ baseUrl })

    isPolling = true

    setInterval(async () => {
        console.log('Polling for tasks...')
        try {
            const searchResponse = await fhirClient.search({
                resourceType: "Task",
                searchParams: {
                    status: "requested",
                    reasonCode: "http://snomed.info/sct|719858009"
                },
                options: {
                    headers: {
                        "Cache-Control": "no-cache"
                    }
                }
            });

            if (searchResponse.entry && searchResponse.entry.length > 0) {
                const tasks = searchResponse.entry.map((entry: BundleEntry<Task>) => entry.resource);
                console.log(`Found ${tasks.length} tasks to update`);

                let updatedCount = 0;
                await Promise.all(tasks.map(async (task: Task) => {
                    try {

                        if (!task.id) throw new Error("No id on Task")

                        const JSONPatch: ReplacePatch = { op: 'replace', path: '/status', value: 'accepted' };

                        await fhirClient.patch({
                            resourceType: 'Task',
                            id: task.id,
                            JSONPatch: [JSONPatch]
                        });
                        updatedCount++;
                    } catch (patchError) {
                        console.error(`Failed to update Task/${task.id}:`, patchError);
                    }
                }));
                console.log(`${updatedCount} tasks updated to accepted`);
            } else {
                console.log('No tasks found to update');
            }
        } catch (searchError) {
            console.error('Failed to search for tasks:', searchError);
        }
    }, 5000);

    return NextResponse.json({ status: "ok" });
}