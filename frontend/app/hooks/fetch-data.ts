import {useEffect, useState, useCallback} from "react";

type FetchDataArgs<T> = {
    queryKey: string[];
    queryFn: () => Promise<T>;
    initialData: T;
    compareFn?: (newData: T, oldData: T) => boolean;
};

export const FetchData = <T>({ queryFn, queryKey, initialData, compareFn }: FetchDataArgs<T>) => {
    const [data, setData] = useState<T>(initialData);
    const [isLoading, setIsLoading] = useState(false);
    const [isError, setIsError] = useState(false);

    const fetchData = useCallback(async () => {
        setIsError(false);
        setIsLoading(true);

        try {
            const result = await queryFn();
            setData(result);
        } catch (error) {
            setIsError(true);
        }

        setIsLoading(false);
    }, [queryFn]);

    useEffect(() => {
        fetchData();
    }, [fetchData, ...queryKey]);

    const refetch = useCallback(async () => {
        try {
            const result = await queryFn();

            if (!compareFn || compareFn(result, data)) {
                setData(result);
            }
        } catch (error) {
            setIsError(true);
        }
    }, [queryFn, data, compareFn]);

    return { data, isLoading, isError, refetch };
};