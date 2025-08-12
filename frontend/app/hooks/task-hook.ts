import Client from "fhir-kit-client";
import {Task} from "fhir/r4";
import {useQuery} from "@tanstack/react-query";
import {fetchTaskById} from "@/lib/fhirUtils";

export type TaskHookArgs = {
    cpsClient: Client;
    taskId: string;
}

export type TaskHookResult = {
    task: Task;
    isLoading: boolean;
    isError: boolean;
}

export default function TaskHook({cpsClient, taskId}: TaskHookArgs ): TaskHookResult {
    const {data, isError, isLoading} = useQuery({
        queryKey: ['task', taskId],
        queryFn: () => fetchTaskById(cpsClient, taskId),
        enabled: !!taskId && !!cpsClient,
    });
    
    return {
        task: data || ({} as Task),
        isLoading,
        isError
    }
}

