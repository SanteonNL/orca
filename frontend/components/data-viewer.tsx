import React, { useEffect } from 'react'
import { Bundle, Task } from 'fhir/r4'
import { getAggregationUrl } from '@/app/actions'
import { Alert, Button } from '@mui/material'
import { RefreshCw } from 'lucide-react'

export default function DataViewer({ task }: { task: Task }) {

    const [data, setData] = React.useState<any[]>([])
    const [aggregationEndpoint, setAggegationEndpoint] = React.useState<string | undefined>()
    const [scpContext, setScpContext] = React.useState<string | null>(null) // used to determine the SCP context of an aggregate request
    const [error, setError] = React.useState<string | null>(null)

    useEffect(() => {
        getAggregationUrl().then((url) => {
            setAggegationEndpoint(url)
        })
    }, [])

    useEffect(() => {
        console.log("Task changed")
        if (!task.basedOn) {
            setError('Task has no basedOn reference')
            return
        }

        const taskWithCarePlanBasedOn = task.basedOn.find(ref => ref.reference?.includes("CarePlan/"))
        if (!taskWithCarePlanBasedOn) {
            setError('Task has no CarePlan as basedOn reference')
            return
        }

        const carePlanId = taskWithCarePlanBasedOn.reference?.replace(/.*CarePlan\/([^?\/]+).*/, "$1")

        if (!carePlanId) {
            setError(`Unable to detect a CarePlan id for Task.basedOn (Task/${task.id})`)
            return
        }

        if (!task.meta?.source) {
            setError(`Task.meta.source is not defined, this is required to determine the SCP context`)
            return
        }

        const taskSourceUrl = new URL(task.meta.source)
        taskSourceUrl.pathname = taskSourceUrl.pathname.replace(/\/Task\/[^/]+/, `/CarePlan/${carePlanId}`)
        setScpContext(taskSourceUrl.toString())
        setError(null)
    }, [task])

    useEffect(() => {

        if (!aggregationEndpoint) {
            return
        }

        const requestUrl = `${aggregationEndpoint}/Observation/_search`;
        console.log(`Sending request to ${requestUrl} with X-Scp-Context: ${scpContext}`);

        if (!scpContext) {
            setError('SCP context is not set');
            return;
        }

        fetch(requestUrl, {
            method: 'POST',
            headers: {
                "X-Scp-Context": scpContext,
                "Content-Type": "application/x-www-form-urlencoded",
            },
        }).then(async (response) => {
            if (!response.ok) {
                setError(`Failed to fetch ${requestUrl}: ${response.statusText}`);
                return
            }

            const result = await response.json() as Bundle

            const entries = result.entry
                ?.filter((entry) => entry.resource?.resourceType === "Observation")
                ?.map((entry) => entry.resource)

            setData(entries || [])
        })
    }, [aggregationEndpoint, scpContext])


    return (
        <div className='w-full'>
            <h2>Data Viewer</h2>
            {error && <Alert severity="error">{error}</Alert>}
            {data.length}
        </div>
    )
}
