import {useCallback, useEffect, useRef} from "react";
import {FetchData} from "@/app/hooks/fetch-data";
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
    const intervalRef = useRef<NodeJS.Timeout | null>(null);
    const queryFnRef = useRef<(() => Promise<TaskProgress>) | null>(null);

    const queryFn = useCallback(async () => {
        return await fetchAllResources(taskId, cpsClient)
    }, [taskId, cpsClient]);

    queryFnRef.current = queryFn;

    const hasDataChanged = (newData: TaskProgress, oldData: TaskProgress): boolean => {
        if (newData.task.status !== oldData.task.status) {
            return true; // Task status has changed, we need to refetch
        }
        return newData.subTasks.length !== oldData.subTasks.length;
    };

    const {data, isLoading, isError, refetch} = FetchData<TaskProgress>({
        queryKey: [taskId],
        queryFn,
        initialData: {
            task: {} as Task,
            questionnaireMap: {},
            subTasks: []
        },
        compareFn: hasDataChanged
    });

    // Set up polling when component mounts
    useEffect(() => {
        // Only start polling if we have a valid task status that should be polled
        const shouldPoll = data.task.status && !['completed', 'failed', 'cancelled', 'rejected', 'accepted'].includes(data.task.status);

        if (shouldPoll && pollingInterval > 0) {
            intervalRef.current = setInterval(() => {
                if (queryFnRef.current) {
                    refetch();
                }
            }, pollingInterval);
        }

        // Cleanup interval on unmount or when polling should stop
        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
        };
    }, [data.task.status, pollingInterval, refetch]);

    return {
        isLoading,
        isError,
        task: data.task,
        questionnaireMap: data.questionnaireMap,
        subTasks: data.subTasks
    };
}

