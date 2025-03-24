import { Bundle, Questionnaire, Task } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { createCpsClient, fetchAllBundlePages } from '../fhirUtils';

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
    onSubTaskSubmit: (callback: any) => void
    fetchAllResources: () => Promise<void>
    refetchTasks: () => void
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
    onSubTaskSubmit: async (callback: any) => {
        //TODO: Should work with EventSource to listen for changes in the Task status

        const selectedTaskId = get().selectedTaskId

        if (!selectedTaskId) return

        const interval = setInterval(async () => {
            const [task, subTasks] = await Promise.all([
                await cpsClient.read({ resourceType: 'Task', id: selectedTaskId }) as Task,
                await fetchSubTasks(selectedTaskId)
            ])

            if (task.status === 'accepted') {
                set({ task, subTasks, primaryTaskCompleted: true })
                clearInterval(interval)

                if (callback) callback()
            } else if (get().subTasks?.length !== subTasks.length) {
                await fetchQuestionnaires(subTasks, set)
                set({ subTasks })
                clearInterval(interval)
                if (callback) callback()
            }

        }, 1000)
    },
    fetchAllResources: async () => {

        try {
            const { loading, selectedTaskId } = get()

            if (!loading && selectedTaskId) {
                set({ loading: true, error: undefined })

                const [task, subTasks] = await Promise.all([
                    await cpsClient.read({ resourceType: 'Task', id: selectedTaskId }) as Task,
                    await fetchSubTasks(selectedTaskId)
                ])

                set({ task, subTasks })
                await fetchQuestionnaires(subTasks, set)
                set({ initialized: true, loading: false, })
            }
        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
    refetchTasks: async () => {
        const selectedTaskId = get().selectedTaskId

        if (!selectedTaskId) return

        const [task, subTasks] = await Promise.all([
            await cpsClient.read({ resourceType: 'Task', id: selectedTaskId }) as Task,
            await fetchSubTasks(selectedTaskId)
        ])

        if (task.status === 'accepted') {
            set({ task, subTasks, primaryTaskCompleted: true })
        } else if (get().subTasks?.length !== subTasks.length) {
            await fetchQuestionnaires(subTasks, set)
            set({ subTasks })
        }
    }
}));

const fetchQuestionnaires = async (subTasks: Task[], set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: false | undefined) => void) => {
    const tmpMap: Record<string, Questionnaire> = {};
    await Promise.all(subTasks.map(async (task: Task) => {
        if (task.input && task.input.length > 0) {
            const input = task.input.find(input => input.valueReference?.reference?.startsWith("Questionnaire"));
            if (input && task.id && input.valueReference?.reference) {
                const questionnaireId = input.valueReference.reference;
                try {
                    const questionnaire = await cpsClient.read({
                        resourceType: "Questionnaire",
                        id: questionnaireId.split("/")[1]
                    }) as Questionnaire;
                    tmpMap[task.id] = questionnaire;
                } catch (error) {
                    set({ error: `Failed to fetch questionnaire: ${error}` });
                }
            }
        }
    }));
    set({ taskToQuestionnaireMap: tmpMap });
}

const fetchSubTasks = async (taskId: string) => {
    const subTaskBundle = await cpsClient.search({
        resourceType: 'Task',
        searchParams: { "part-of": `Task/${taskId}` },
        headers: { "Cache-Control": "no-cache" },
        // @ts-ignore
        options: { postSearch: true }
    }) as Bundle<Task>
    return await fetchAllBundlePages(cpsClient, subTaskBundle)
}

const useTaskProgressStore = () => {
    const store = taskProgressStore();

    const loading = taskProgressStore(state => state.loading);
    const initialized = taskProgressStore(state => state.initialized);
    const selectedTaskId = taskProgressStore(state => state.selectedTaskId);
    const fetchAllResources = taskProgressStore(state => state.fetchAllResources);

    useEffect(() => {
        if (!loading && !initialized && selectedTaskId) {
            fetchAllResources()
        }
    }, [selectedTaskId, loading, initialized, fetchAllResources]);

    return store;
};

export default useTaskProgressStore;
