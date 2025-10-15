import {useQuery} from "@tanstack/react-query";
import {Questionnaire, Task} from "fhir/r4";
import {fetchAllResources, TaskProgress} from "@/lib/fhirUtils";
import Client from "fhir-kit-client";

type TaskProgressArgs = {
    taskId: string;
    cpsClient: Client;
    pollingInterval: number;
}

type FetchAllResources = {
    task: Task;
    questionnaireMap: Record<string, Questionnaire>;
    subTasks: Task[]
    isLoading: boolean;
    isError: boolean;
}



export default function TaskProgressHook({taskId, cpsClient, pollingInterval}: TaskProgressArgs): FetchAllResources {
    const {data, isLoading, isError} = useQuery({
        queryKey: ['task-progress', taskId],
        queryFn: () => fetchAllResources(taskId, cpsClient),
        enabled: !!taskId && !!cpsClient,
        refetchInterval: (query) => {
            const taskData = query.state.data as TaskProgress | undefined;
            const shouldPoll = !!taskData?.task && !!taskData.task.status && 
                !['completed', 'failed', 'cancelled', 'rejected'].includes(taskData.task.status);
            
            return shouldPoll && pollingInterval > 0 ? pollingInterval : false;
        }
    });

    const defaultData: TaskProgress = {
        task: {} as Task,
        questionnaireMap: {},
        subTasks: []
    };

    return {
        isLoading,
        isError,
        task: data?.task || defaultData.task,
        questionnaireMap: data?.questionnaireMap || defaultData.questionnaireMap,
        subTasks: data?.subTasks || defaultData.subTasks
    };
}

