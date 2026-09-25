export interface paths {
    "/api/v1/auth/forgot-password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Request a password reset */
        post: operations["auth-request-password-reset"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/login": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Log in */
        post: operations["auth-login"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Revoke a refresh token */
        post: operations["auth-logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/refresh": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Rotate a refresh token */
        post: operations["auth-refresh"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/register": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Register a user */
        post: operations["auth-register"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/reset-password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Set a new password */
        post: operations["auth-reset-password"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/verify-email": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Confirm an email address */
        post: operations["auth-verify-email"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Check API health */
        get: operations["health-check"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get candidate profile */
        get: operations["profile-get"];
        /** Update candidate profile */
        put: operations["profile-put"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile/positions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get desired positions */
        get: operations["profile-positions-get"];
        /** Replace desired positions */
        put: operations["profile-positions-put"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile/preferences": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get job preferences */
        get: operations["profile-preferences-get"];
        /** Update job preferences */
        put: operations["profile-preferences-put"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile/skills": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get candidate skills */
        get: operations["profile-skills-get"];
        /** Replace candidate skills */
        put: operations["profile-skills-put"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile/sources": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get source preferences */
        get: operations["profile-sources-get"];
        /** Update source preferences */
        put: operations["profile-sources-put"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ready": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Check required backend dependencies */
        get: operations["readiness-check"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List candidate resumes */
        get: operations["resume-list"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/upload-url": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Create a private resume upload URL */
        post: operations["resume-upload-url"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get a resume and short-lived download URL */
        get: operations["resume-get"];
        put?: never;
        post?: never;
        /** Delete a resume */
        delete: operations["resume-delete"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/{id}/analysis": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get parsed resume and profile suggestions */
        get: operations["resume-analysis"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/{id}/complete": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Verify a resume upload */
        post: operations["resume-complete"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/{id}/suggestions/{suggestionID}/accept": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Accept a profile suggestion */
        post: operations["resume-suggestion-accept"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/resumes/{id}/suggestions/{suggestionID}/reject": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Reject a profile suggestion */
        post: operations["resume-suggestion-reject"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        DetectedPosition: {
            /** Format: double */
            confidence: number;
            title: string;
        };
        DetectedSkill: {
            /** Format: double */
            confidence: number;
            name: string;
        };
        ErrorDetail: {
            /** @description Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id' */
            location?: string;
            /** @description Error message text */
            message?: string;
            /** @description The value at the given location */
            value?: unknown;
        };
        ErrorModel: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ErrorModel.json
             */
            readonly $schema?: string;
            /**
             * @description A human-readable explanation specific to this occurrence of the problem.
             * @example Property foo is required but is missing.
             */
            detail?: string;
            /** @description Optional list of individual error details */
            errors?: components["schemas"]["ErrorDetail"][] | null;
            /**
             * Format: uri
             * @description A URI reference that identifies the specific occurrence of the problem.
             * @example https://example.com/error-log/abc123
             */
            instance?: string;
            /**
             * Format: int64
             * @description HTTP status code
             * @example 400
             */
            status?: number;
            /**
             * @description A short, human-readable summary of the problem type. This value should not change between occurrences of the error.
             * @example Bad Request
             */
            title?: string;
            /**
             * Format: uri
             * @description A URI reference to human-readable documentation for the error.
             * @default about:blank
             * @example https://example.com/errors/example
             */
            type: string;
        };
        HealthOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/HealthOutputBody.json
             */
            readonly $schema?: string;
            status: string;
        };
        LoginInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/LoginInputBody.json
             */
            readonly $schema?: string;
            email: string;
            password: string;
        };
        Money: {
            /** Format: int64 */
            amount: number;
            currency: string;
        };
        ParsedResume: {
            positions: components["schemas"]["DetectedPosition"][] | null;
            resume_id: string;
            skills: components["schemas"]["DetectedSkill"][] | null;
            text?: string;
            /** Format: int64 */
            total_experience_months: number;
        };
        PositionsOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/PositionsOutputBody.json
             */
            readonly $schema?: string;
            positions: string[] | null;
        };
        PositionsPutInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/PositionsPutInputBody.json
             */
            readonly $schema?: string;
            positions: string[] | null;
        };
        Preferences: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Preferences.json
             */
            readonly $schema?: string;
            allowed_regions: string[] | null;
            employment_types: string[] | null;
            excluded_countries: string[] | null;
            /** Format: int64 */
            maximum_job_age_days: number;
            /** Format: int64 */
            minimum_match_score: number;
            minimum_salary?: components["schemas"]["Money"];
            notifications_enabled: boolean;
            remote_policies: string[] | null;
        };
        Profile: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Profile.json
             */
            readonly $schema?: string;
            city: string;
            country: string;
            desired_salary?: components["schemas"]["Money"];
            /** Format: int64 */
            experience_years: number;
            first_name: string;
            last_name: string;
            seniority: string;
            timezone: string;
        };
        RefreshInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RefreshInputBody.json
             */
            readonly $schema?: string;
            refresh_token: string;
        };
        RegisterInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RegisterInputBody.json
             */
            readonly $schema?: string;
            email: string;
            password: string;
        };
        RegisterOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RegisterOutputBody.json
             */
            readonly $schema?: string;
            user_id: string;
        };
        RequestResetInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RequestResetInputBody.json
             */
            readonly $schema?: string;
            email: string;
        };
        ResetPasswordInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResetPasswordInputBody.json
             */
            readonly $schema?: string;
            password: string;
            token: string;
        };
        Resume: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Resume.json
             */
            readonly $schema?: string;
            content_type: string;
            /** Format: date-time */
            created_at: string;
            file_name: string;
            id: string;
            /** Format: int64 */
            size: number;
            status: string;
            /** Format: date-time */
            updated_at: string;
        };
        ResumeAnalysisOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResumeAnalysisOutputBody.json
             */
            readonly $schema?: string;
            analysis: components["schemas"]["ParsedResume"];
            suggestions: components["schemas"]["Suggestion"][] | null;
        };
        ResumeDownloadOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResumeDownloadOutputBody.json
             */
            readonly $schema?: string;
            download_url?: string;
            resume: components["schemas"]["Resume"];
        };
        ResumeUploadInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResumeUploadInputBody.json
             */
            readonly $schema?: string;
            content_type: string;
            file_name: string;
            /** Format: int64 */
            size: number;
        };
        ResumeUploadOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResumeUploadOutputBody.json
             */
            readonly $schema?: string;
            resume: components["schemas"]["Resume"];
            upload_url: string;
        };
        ResumesOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ResumesOutputBody.json
             */
            readonly $schema?: string;
            resumes: components["schemas"]["Resume"][] | null;
        };
        Skill: {
            level?: string;
            name: string;
            /** Format: double */
            years?: number;
        };
        SkillsOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SkillsOutputBody.json
             */
            readonly $schema?: string;
            skills: components["schemas"]["Skill"][] | null;
        };
        SkillsPutInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SkillsPutInputBody.json
             */
            readonly $schema?: string;
            skills: components["schemas"]["Skill"][] | null;
        };
        SourcePreference: {
            enabled: boolean;
            source_id: string;
        };
        SourcePreferencesOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SourcePreferencesOutputBody.json
             */
            readonly $schema?: string;
            sources: components["schemas"]["SourcePreference"][] | null;
        };
        SourcePreferencesPutInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SourcePreferencesPutInputBody.json
             */
            readonly $schema?: string;
            sources: components["schemas"]["SourcePreference"][] | null;
        };
        Suggestion: {
            /** Format: double */
            confidence: number;
            id: string;
            kind: string;
            resume_id: string;
            status: string;
            value: string;
        };
        TokenOutputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/TokenOutputBody.json
             */
            readonly $schema?: string;
            access_expires_at: string;
            access_token: string;
            refresh_expires_at: string;
            refresh_token: string;
        };
        VerifyEmailInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/VerifyEmailInputBody.json
             */
            readonly $schema?: string;
            token: string;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    "auth-request-password-reset": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RequestResetInputBody"];
            };
        };
        responses: {
            /** @description Accepted */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-login": {
        parameters: {
            query?: never;
            header?: {
                "User-Agent"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["LoginInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TokenOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RefreshInputBody"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-refresh": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RefreshInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TokenOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-register": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RegisterInputBody"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RegisterOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-reset-password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ResetPasswordInputBody"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "auth-verify-email": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["VerifyEmailInputBody"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "health-check": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HealthOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Profile"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-put": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Profile"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Profile"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-positions-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PositionsOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-positions-put": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["PositionsPutInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PositionsOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-preferences-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Preferences"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-preferences-put": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Preferences"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Preferences"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-skills-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SkillsOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-skills-put": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SkillsPutInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SkillsOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-sources-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SourcePreferencesOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "profile-sources-put": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SourcePreferencesPutInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SourcePreferencesOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "readiness-check": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HealthOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-list": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ResumesOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-upload-url": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ResumeUploadInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ResumeUploadOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-get": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ResumeDownloadOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-delete": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-analysis": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ResumeAnalysisOutputBody"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-complete": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Resume"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-suggestion-accept": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
                suggestionID: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "resume-suggestion-reject": {
        parameters: {
            query?: never;
            header?: {
                Authorization?: string;
            };
            path: {
                id: string;
                suggestionID: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
}
