import {useCallback, useEffect, useRef} from "react";
import {FetchData} from "@/app/hooks/fetch-data";
import {Bundle, Questionnaire, Task} from "fhir/r4";
import Client from "fhir-kit-client";
import {fetchAllBundlePages} from "@/lib/fhirUtils";

type TaskData = {
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

type TaskProgress = {
    task: Task;
    questionnaireMap: Record<string, Questionnaire>;
    subTasks: Task[]
}

export default function TaskProgressHook({taskId, cpsClient, pollingInterval}: TaskData): FetchAllResources {
    const intervalRef = useRef<NodeJS.Timeout | null>(null);
    const queryFnRef = useRef<(() => Promise<TaskProgress>) | null>(null);

    const queryFn = useCallback(async () => {
        return await fetchAllResources(taskId, cpsClient)
    }, [taskId, cpsClient]);

    queryFnRef.current = queryFn;

    const hasDataChanged = (newData: TaskProgress, oldData: TaskProgress): boolean => {
        if(newData.task.status !== oldData.task.status) {
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

const fetchAllResources = async (taskId: string, cpsClient: Client): Promise<TaskProgress> => {
    const [task, subTasks] = await Promise.all([
        await cpsClient.read({resourceType: 'Task', id: taskId}) as Task,
        await fetchSubTasks(cpsClient, taskId)
    ])

    const questionnaireMap = await fetchQuestionnaires(cpsClient, subTasks);
    return {task, questionnaireMap, subTasks};
}


const fetchQuestionnaires = async (cpsClient: Client, subTasks: Task[]) => {
    const questionnaireMap: Record<string, Questionnaire> = {};
    await Promise.all(subTasks.map(async (task: Task) => {
        if (task.input && task.input.length > 0) {
            const input = task.input.find(input => input.valueReference?.reference?.startsWith("Questionnaire"));
            if (input && task.id && input.valueReference?.reference) {
                const questionnaireId = input.valueReference.reference;
                try {
                    questionnaireMap[task.id] = await cpsClient.read({
                        resourceType: "Questionnaire",
                        id: questionnaireId.split("/")[1]
                    }) as Questionnaire;
                } catch (error) {
                    throw new Error(`Failed to fetch questionnaire: ${error}`);
                }
            }
        }
    }));
    return questionnaireMap
}


const fetchSubTasks = async (cpsClient: Client, taskId: string) => {
    try {
        const subTaskBundle = await cpsClient.search({
            resourceType: 'Task',
            searchParams: {"part-of": `Task/${taskId}`},
            headers: {"Cache-Control": "no-cache"},
            // @ts-ignore
            options: {postSearch: true}
        }) as Bundle<Task>;
        const subTasks = await fetchAllBundlePages(cpsClient, subTaskBundle);
        if (Array.isArray(subTasks) && subTasks.length > 0) {
            return subTasks;
        }
    } catch (error) {
        throw new Error(`Failed to fetch sub-tasks: ${error}`);
    }
    return [];
}