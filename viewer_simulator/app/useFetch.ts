import { useCallback, useEffect, useState } from 'react';

export default function useFetch<T>(url: string) {
    const [loading, setLoading] = useState(false);
    const [data, setData] = useState<T | null>(null);
    const loadData = useCallback(() => {
        setLoading(true);
        fetch(url)
            .then((response) => response.json())
            .then((data) => {
                setData(data);
                setLoading(false);
            })
            .catch((error) => console.error(error));
    }, [url]);

    useEffect(() => {
        loadData();
    }, [loadData]);

    return { loading, data };
}
