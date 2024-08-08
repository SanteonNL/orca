import { Bundle, Task } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { createCpsClient, fetchAllBundlePages } from '../fhirUtils';

const cpsClient = createCpsClient()

interface StoreState {
    initialized: boolean
    loading: boolean
    error?: string
    task?: Task
    selectedTaskId?: string
    subTasks?: Task[]
    setSelectedTaskId: (taskId: string) => void
    setTask: (task?: Task) => void
    setSubTasks: (subTasks: Task[]) => void
    fetchAllResources: () => Promise<void>
}

const taskProgressStore = create<StoreState>((set, get) => ({
    initialized: false,
    loading: false,
    task: undefined,
    subTasks: undefined,
    error: undefined,
    setSelectedTaskId: (taskId: string) => {
        set({ selectedTaskId: taskId })
    },
    setTask: (task?: Task) => {
        set({ task });
    },
    setSubTasks: (subTasks: Task[]) => {
        set({ subTasks })
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
            }
            set({ initialized: true, loading: false, })

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
}));

const fetchSubTasks = async (taskId: string) => {
    const subTaskBundle = await cpsClient.search({ resourceType: 'Task', searchParams: { "based-on": `Task/${taskId}` } }) as Bundle<Task>
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
