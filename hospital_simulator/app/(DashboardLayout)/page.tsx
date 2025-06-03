'use client'
import {useRouter} from "next/navigation";
import {useEffect} from "react";

const Dashboard = () => {
    const { push } = useRouter();

    useEffect(() => {
        push('/patients');
    }, []);
    return <p></p>;
}

export default Dashboard;
