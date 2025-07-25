import {Bundle, Questionnaire, Task} from 'fhir/r4';
import {useEffect} from 'react';
import {create} from 'zustand';
import {createCpsClient, fetchAllBundlePages} from '../fhirUtils';

const cpsClient = createCpsClient()
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
    setSelectedTaskId: (taskId: string) => void
    setTask: (task?: Task) => void
    nextStep: () => void
    setSubTasks: (subTasks: Task[]) => void
    fetchAllResources: (selectedTaskId: string) => Promise<void>
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
        try {
            set({ loading: true, error: undefined })

            const [task, subTasks] = await Promise.all([
                await cpsClient.read({ resourceType: 'Task', id: selectedTaskId }) as Task,
                await fetchSubTasks(selectedTaskId)
            ])
            set({ task, subTasks })
            const questionnaireMap = await fetchQuestionnaires(subTasks, set)

            set({ taskToQuestionnaireMap: questionnaireMap });
            set({ initialized: true, loading: false, })

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    }
}));

const fetchQuestionnaires = async (subTasks: Task[], set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: false | undefined) => void) => {
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

    useEffect(() => {
        if (!loading && !initialized && selectedTaskId) {
            fetchAllResources(selectedTaskId)
        }
    }, [selectedTaskId, loading, initialized, fetchAllResources]);

    return store;
};

export default useTaskProgressStore;
