import { getEnrollmentUrl } from '@/utils/config';
import { Button } from '@mui/material';
import { IconCloudDataConnection } from '@tabler/icons-react';
import React from 'react';
import {Identifier} from "fhir/r4";

interface Props {
    patientId: string,
    serviceRequestId: string,
    title?: string
    width?: number
    height?: number
    callback?(): void
}

const EnrollmentPopup: React.FC<Props> = ({ patientId, serviceRequestId, title = "Enrollment Process", width = 1200, height = 900, callback }) => {
    const openPopup = async () => {

        const url = await getEnrollmentUrl(patientId, serviceRequestId)

        const left = (window.screen.width / 2) - (width / 2);
        const top = (window.screen.height / 2) - (height / 2);
        window.open(
            url,
            title,
            `toolbar=no, location=no, directories=no, status=no, menubar=no, scrollbars=yes, resizable=yes, copyhistory=no, width=${width}, height=${height}, top=${top}, left=${left}`
        );

        if (callback) callback()
    };

    return (
        <Button variant='outlined' onClick={openPopup}>
            <IconCloudDataConnection />
        </Button>
    );
};

export default EnrollmentPopup;
