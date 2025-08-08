import Client from "fhir-kit-client";
import {Task} from "fhir/r4";
import {FetchData} from "@/app/hooks/fetch-data";
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
    const {data, isError, isLoading} =  FetchData<Task>({
        queryKey: [taskId],
        queryFn: async ()=> {
            return await fetchTaskById(cpsClient, taskId);
        },
        initialData: {} as Task
    });
    return {
        task: data,
        isLoading,
        isError
    }
}

