import { Bundle, Questionnaire, Task } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { createCpsClient, fetchAllBundlePages } from '../fhirUtils';

const cpsClient = createCpsClient()
// A module-level variable, to ensure only one SSE subscription is active (`use` hook for this store is used in multiple places).
let globalEventSource: EventSource | null = null;
interface StoreState {
    initialized: boolean
    loading: boolean
    error?: string
    task?: Task
    currentStep: number
    primaryTaskCompleted?: boolean
    selectedTaskId?: string
    subTasks?: Task[]
    taskToQuestionnaireMap?: Record<string, Questionnaire>
    questionnaireToResponseMap?: Record<string, Questionnaire>
    eventSourceConnected: boolean
    setSelectedTaskId: (taskId: string) => void
    setTask: (task?: Task) => void
    nextStep: () => void
    setSubTasks: (subTasks: Task[]) => void
    fetchAllResources: (selectedTaskId: string) => Promise<void>
    setEventSourceConnected: (connected: boolean) => void
}

const taskProgressStore = create<StoreState>((set, get) => ({
    initialized: false,
    loading: false,
    task: undefined,
    currentStep: 0,
    primaryTaskCompleted: false,
    subTasks: undefined,
    taskToQuestionnaireMap: undefined,
    error: undefined,
    eventSourceConnected: false,
    setSelectedTaskId: (taskId: string) => {
        console.log(`Setting selected task ID: ${taskId}`);
        set({ selectedTaskId: taskId })
    },
    setTask: (task?: Task) => {
        set({ task });
    },
    nextStep: () => {
        set({ currentStep: get().currentStep + 1 })//todo: limit to the number of tasks
    },
    setSubTasks: (subTasks: Task[]) => {
        set({ subTasks })
    },
    fetchAllResources: async (selectedTaskId: string) => {
        console.log("Fetching all resources for the selected task...");
        try {
            set({ loading: true, error: undefined })

            const [task, subTasks] = await Promise.all([
                await cpsClient.read({ resourceType: 'Task', id: selectedTaskId }) as Task,
                await fetchSubTasks(selectedTaskId)
            ])

            console.log(`Fetched Task: ${JSON.stringify(task)}`);
            console.log(`Fetched SubTasks: ${JSON.stringify(subTasks)}`);

            set({ task, subTasks })
            const questionnaireMap = await fetchQuestionnaires(subTasks, set)

            console.log(`Fetched Questionnaires: ${JSON.stringify(questionnaireMap)}`);
            set({ taskToQuestionnaireMap: questionnaireMap });
            set({ initialized: true, loading: false, })

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
    setEventSourceConnected: (connected: boolean) => {
        set({ eventSourceConnected: connected })
    }
}));

const fetchQuestionnaires = async (subTasks: Task[], set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: false | undefined) => void) => {
    const questionnaireMap: Record<string, Questionnaire> = {};
    await Promise.all(subTasks.map(async (task: Task) => {
        if (task.input && task.input.length > 0) {
            const input = task.input.find(input => input.valueReference?.reference?.startsWith("Questionnaire"));
            console.log(`Found the input for Task ${task.id}: ${JSON.stringify(input)}`);
            if (input && task.id && input.valueReference?.reference) {
                const questionnaireId = input.valueReference.reference;
                try {
                    const questionnaire = await cpsClient.read({
                        resourceType: "Questionnaire",
                        id: questionnaireId.split("/")[1]
                    }) as Questionnaire;
                    console.log(`Found the questionnaire  ${task.id}: ${JSON.stringify(questionnaire)}`);
                    questionnaireMap[task.id] = questionnaire;
                } catch (error) {
                    set({ error: `Failed to fetch questionnaire: ${error}` });
                }
            }
        }
    }));
    return questionnaireMap
}

const fetchSubTasks = async (taskId: string) => {
    for (let attempts = 0; attempts < 3; attempts++) {
        const subTaskBundle = await cpsClient.search({
            resourceType: 'Task',
            searchParams: { "part-of": `Task/${taskId}` },
            headers: { "Cache-Control": "no-cache" },
            // @ts-ignore
            options: { postSearch: true }
        }) as Bundle<Task>;
        const subTasks = await fetchAllBundlePages(cpsClient, subTaskBundle);

        if (Array.isArray(subTasks) && subTasks.length > 0) {
            return subTasks;
        }
        const delay = 200 * Math.pow(2, attempts);
        await new Promise(res => setTimeout(res, delay));
    }
    return [];
}

const useTaskProgressStore = () => {
    const store = taskProgressStore();

    const loading = taskProgressStore(state => state.loading);
    const initialized = taskProgressStore(state => state.initialized);
    const selectedTaskId = taskProgressStore(state => state.selectedTaskId);
    const fetchAllResources = taskProgressStore(state => state.fetchAllResources);
    const setEventSourceConnected = taskProgressStore(state => state.setEventSourceConnected);

    useEffect(() => {
        if (!loading && !initialized && selectedTaskId) {
            fetchAllResources(selectedTaskId)
        }
    }, [selectedTaskId, loading, initialized, fetchAllResources]);

    useEffect(() => {
        // Only subscribe if we have a selectedTaskId and no active global subscription yet.
        if (!selectedTaskId || globalEventSource) return

        globalEventSource = new EventSource(`/orca/cpc/subscribe/fhir/Task/${selectedTaskId}`);

        globalEventSource.onopen = () => {
            setEventSourceConnected(true);
        }

        globalEventSource.onerror = (error) => {
            setEventSourceConnected(false);
            console.error(`Error in global EventSource: ${JSON.stringify(error)}`);
        };

        globalEventSource.onmessage = (event) => {
            const task = JSON.parse(event.data) as Task;
            // Detect if it's the primary Task or a subtask.
            if (task.id === selectedTaskId) {
                taskProgressStore.setState({ task });
            } else if (task.partOf?.some(ref => ref.reference === `Task/${selectedTaskId}`)) {
                taskProgressStore.setState((state) => {
                    const currentSubTasks = state.subTasks || [];
                    const index = currentSubTasks.findIndex(subTask => subTask.id === task.id);
                    if (index === -1) {
                        // If the subtask is new, add it to the array.
                        return { subTasks: [...currentSubTasks, task] };
                    } else {
                        // If the subtask exists, update it.
                        const updatedSubTasks = [...currentSubTasks];
                        updatedSubTasks[index] = task;
                        return { subTasks: updatedSubTasks };
                    }
                });
            }
        };

        // Clean up the global subscription when the component unmounts.
        return () => {
            globalEventSource?.close();
            globalEventSource = null;
            setEventSourceConnected(false);
        };
    }, [selectedTaskId, setEventSourceConnected]);

    return store;
};

export default useTaskProgressStore;
